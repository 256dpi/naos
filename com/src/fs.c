#include <naos/fs.h>
#include <naos/msg.h>
#include <naos/sys.h>

#include <errno.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <sys/dirent.h>
#include <sys/syslimits.h>
#include <esp_vfs_fat.h>
#include <mbedtls/sha256.h>

#define NAOS_FS_ENDPOINT 0x03
#define NAOS_FS_MAX_FILES 4

typedef enum {
  NAOS_FS_CMD_STAT,
  NAOS_FS_CMD_LIST,
  NAOS_FS_CMD_OPEN,
  NAOS_FS_CMD_READ,
  NAOS_FS_CMD_WRITE,
  NAOS_FS_CMD_CLOSE,
  NAOS_FS_CMD_RENAME,
  NAOS_FS_CMD_REMOVE,
  NAOS_FS_CMD_SHA256,
} naos_fs_cmd_t;

typedef enum {
  NAOS_FS_REPLY_ERROR,
  NAOS_FS_REPLY_INFO,
  NAOS_FS_REPLY_CHUNK,
  NAOS_FS_REPLY_SHA256,
} naos_fs_reply_t;

typedef enum {
  NAOS_FS_OPEN_FLAG_CREATE = 1 << 0,
  NAOS_FS_OPEN_FLAG_APPEND = 1 << 1,
  NAOS_FS_OPEN_FLAG_TRUNCATE = 1 << 2,
  NAOS_FS_OPEN_FLAG_EXCLUSIVE = 1 << 3,
} naos_fs_open_flags_t;

typedef enum {
  NAOS_FS_WRITE_FLAG_SILENT = 1 << 0,
  NAOS_FS_WRITE_FLAG_SEQUENTIAL = 1 << 1,
} naos_fs_write_flags_t;

typedef struct {
  bool active;
  int fd;
  uint16_t sid;
  int64_t ts;
  uint32_t off;
} naos_fs_file_t;

static naos_mutex_t naos_fs_mutex = 0;
static naos_fs_file_t naos_fs_files[NAOS_FS_MAX_FILES] = {0};
static naos_fs_config_t naos_fs_config = {0};
static char naos_fs_path[256] = {0};

static const char *naos_fs_concat_path(const char *path) {
  // check root
  if (naos_fs_config.root == NULL) {
    return path;
  }

  // build path
  naos_fs_path[0] = 0;
  strcat(naos_fs_path, naos_fs_config.root);
  strcat(naos_fs_path, path);

  return naos_fs_path;
}

static naos_msg_reply_t naos_fs_send_error(uint16_t session, int error) {
  // reply structure:
  // TYPE (1) | ERRNO (1)

  // prepare data
  uint8_t data[] = {NAOS_FS_REPLY_ERROR, error};

  // send error
  naos_msg_send((naos_msg_t){
      .session = session,
      .endpoint = NAOS_FS_ENDPOINT,
      .data = data,
      .len = 2,
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_fs_handle_stat(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check path
  if (msg.len == 0 || msg.data[0] != '/') {
    return NAOS_MSG_INVALID;
  }

  // get path
  const char *path = naos_fs_concat_path((const char *)msg.data);

  // stat path
  struct stat info;
  int ret = stat(path, &info);
  if (ret != 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // get info
  bool is_dir = S_ISDIR(info.st_mode);
  uint32_t size = info.st_size;

  // reply structure:
  // TYPE (1) | IS_DIR (1) | SIZE (4)

  // prepare data
  uint8_t data[6] = {NAOS_FS_REPLY_INFO, is_dir};
  memcpy(&data[2], &size, sizeof(size));

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_FS_ENDPOINT,
      .data = data,
      .len = 6,
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_fs_handle_list(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check path
  if (msg.len == 0 || msg.data[0] != '/') {
    return NAOS_MSG_INVALID;
  }

  // get root
  const char *root = naos_fs_concat_path((const char *)msg.data);

  // open directory
  DIR *dir = opendir(root);
  if (dir == NULL) {
    return naos_fs_send_error(msg.session, errno);
  }

  // prepare path
  char path[PATH_MAX];

  // iterate over directory
  struct dirent *entry;
  while ((entry = readdir(dir)) != NULL) {
    // skip the "." and ".." entries
    if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) {
      continue;
    }

    // join path
    snprintf(path, sizeof(path), "%s/%s", root, entry->d_name);

    // stat file
    struct stat info;
    int ret = stat(path, &info);
    if (ret != 0) {
      closedir(dir);
      return naos_fs_send_error(msg.session, errno);
    }

    // get info
    bool is_dir = S_ISDIR(info.st_mode);
    uint32_t size = info.st_size;

    // reply structure:
    // TYPE (1) | IS_DIR (1) | SIZE (4) | NAME (*)

    // determine length
    size_t length = 6 + strlen(entry->d_name);

    // prepare data
    uint8_t *data = calloc(length, 1);
    data[0] = NAOS_FS_REPLY_INFO;
    data[1] = is_dir;
    memcpy(&data[2], &size, sizeof(size));
    memcpy(&data[6], entry->d_name, strlen(entry->d_name));

    // send reply
    naos_msg_send((naos_msg_t){
        .session = msg.session,
        .endpoint = NAOS_FS_ENDPOINT,
        .data = data,
        .len = length,
    });

    // free data
    free(data);
  }

  // close directory
  closedir(dir);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_open(naos_msg_t msg) {
  // command structure:
  // FLAGS (1) | PATH (*)

  // check path
  if (msg.len <= 1 || msg.data[1] != '/') {
    return NAOS_MSG_INVALID;
  }

  // close already open files
  for (size_t i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (naos_fs_files[i].active && naos_fs_files[i].sid == msg.session) {
      close(naos_fs_files[i].fd);
      naos_fs_files[i].active = false;
    }
  }

  // find first free file
  naos_fs_file_t *file = NULL;
  for (size_t i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (!naos_fs_files[i].active) {
      file = &naos_fs_files[i];
      break;
    }
  }
  if (file == NULL) {
    return naos_fs_send_error(msg.session, ENFILE);
  }

  // get flags
  naos_fs_open_flags_t flags = msg.data[0];

  // prepare open flags
  int open_flags = O_RDWR;
  if (flags & NAOS_FS_OPEN_FLAG_CREATE) {
    open_flags |= O_CREAT;
  }
  if (flags & NAOS_FS_OPEN_FLAG_APPEND) {
    open_flags |= O_APPEND;
  }
  if (flags & NAOS_FS_OPEN_FLAG_TRUNCATE) {
    open_flags |= O_TRUNC;
  }
  if (flags & NAOS_FS_OPEN_FLAG_EXCLUSIVE) {
    open_flags |= O_EXCL;
  }

  // get path
  const char *path = naos_fs_concat_path((const char *)(msg.data + 1));

  // create file
  int fd = open(path, open_flags, 0644);
  if (fd < 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // set file
  file->active = true;
  file->fd = fd;
  file->sid = msg.session;
  file->ts = naos_millis();
  file->off = 0;

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_read(naos_msg_t msg) {
  // command structure:
  // OFFSET (4) | LENGTH (4)

  // check path
  if (msg.len != 8) {
    return NAOS_MSG_INVALID;
  }

  // find file
  naos_fs_file_t *file = NULL;
  for (size_t i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (naos_fs_files[i].active && naos_fs_files[i].sid == msg.session) {
      file = &naos_fs_files[i];
      break;
    }
  }
  if (file == NULL) {
    return naos_fs_send_error(msg.session, EBADF);
  }

  // get offset and length
  uint32_t offset;
  uint32_t length;
  memcpy(&offset, msg.data, sizeof(offset));
  memcpy(&length, &msg.data[4], sizeof(length));

  // stat file
  struct stat info;
  if (fstat(file->fd, &info) != 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // seek to offset
  off_t ret = lseek(file->fd, offset, SEEK_SET);
  if (ret < 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // determine length if zero or limit length
  if (length == 0) {
    if (info.st_size > offset) {
      length = info.st_size - offset;
    }
  } else if (offset + length > info.st_size) {
    length = info.st_size - offset;
  }

  // determine max chunk size
  size_t max_chunk_size = naos_msg_get_mtu(msg.session) - 16;
  if (max_chunk_size > length) {
    max_chunk_size = length;
  }

  // reply structure:
  // TYPE (1) | OFFSET (4) | DATA (*)

  // prepare data
  uint8_t *data = calloc(5 + max_chunk_size, 1);
  data[0] = NAOS_FS_REPLY_CHUNK;

  // read and reply with chunks
  off_t total = 0;
  while (total < length) {
    // determine chunk size
    size_t chunk_size = (length - total) < max_chunk_size ? (length - total) : max_chunk_size;

    // write chunk offset
    uint32_t chunk_offset = offset + total;
    memcpy(&data[1], &chunk_offset, sizeof(chunk_offset));

    // read chunk
    ret = read(file->fd, data + 5, chunk_size);
    if (ret < 0) {
      free(data);
      return naos_fs_send_error(msg.session, errno);
    }

    // send reply
    naos_msg_send((naos_msg_t){
        .session = msg.session,
        .endpoint = NAOS_FS_ENDPOINT,
        .data = data,
        .len = 5 + ret,
    });

    // increment total
    total += ret;

    // update timestamp
    file->ts = naos_millis();

    // yield to system
    NAOS_UNLOCK(naos_fs_mutex);
    naos_delay(1);
    NAOS_LOCK(naos_fs_mutex);
  }

  // free data
  free(data);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_write(naos_msg_t msg) {
  // command structure:
  // FLAGS (1) | OFFSET (4) | DATA (*)

  // check path
  if (msg.len <= 5) {
    return NAOS_MSG_INVALID;
  }

  // get flags
  naos_fs_write_flags_t flags = msg.data[0];
  bool silent = flags & NAOS_FS_WRITE_FLAG_SILENT;
  bool sequential = flags & NAOS_FS_WRITE_FLAG_SEQUENTIAL;

  // get offset
  uint32_t offset;
  memcpy(&offset, msg.data + 1, sizeof(offset));

  // find file
  naos_fs_file_t *file = NULL;
  for (size_t i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (naos_fs_files[i].active && naos_fs_files[i].sid == msg.session) {
      file = &naos_fs_files[i];
      break;
    }
  }
  if (file == NULL) {
    return silent ? NAOS_MSG_OK : naos_fs_send_error(msg.session, EBADF);
  }

  // verify sequential offset
  if (sequential && offset != file->off) {
    return silent ? NAOS_MSG_OK : naos_fs_send_error(msg.session, EINVAL);
  }

  // otherwise, seek random offset
  if (!sequential) {
    off_t ret = lseek(file->fd, offset, SEEK_SET);
    if (ret < 0) {
      return silent ? NAOS_MSG_OK : naos_fs_send_error(msg.session, errno);
    }
    file->off = offset;
  }

  // write all data
  uint32_t total = 0;
  while (total < msg.len - 5) {
    ssize_t ret = write(file->fd, msg.data + 5 + total, msg.len - 5 - total);
    if (ret < 0) {
      return silent ? NAOS_MSG_OK : naos_fs_send_error(msg.session, errno);
    }
    total += ret;
  }

  // update timestamp and offset
  file->ts = naos_millis();
  file->off = offset + total;

  return silent ? NAOS_MSG_OK : NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_close(naos_msg_t msg) {
  // check msg
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // find file
  naos_fs_file_t *file = NULL;
  for (size_t i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (naos_fs_files[i].active && naos_fs_files[i].sid == msg.session) {
      file = &naos_fs_files[i];
      break;
    }
  }
  if (file == NULL) {
    return naos_fs_send_error(msg.session, EBADF);
  }

  // close file
  close(file->fd);

  // reset descriptor
  *file = (naos_fs_file_t){0};

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_rename(naos_msg_t msg) {
  // command structure:
  // FROM (*) | 0 | TO (*)

  // get lengths
  size_t from_len = strlen((const char *)msg.data);
  size_t to_len = strlen((const char *)&msg.data[from_len + 1]);
  if (from_len == 0 || to_len == 0) {
    return NAOS_MSG_INVALID;
  } else if (from_len + 1 + to_len > msg.len) {
    return NAOS_MSG_INVALID;
  } else if (msg.data[0] != '/' || msg.data[from_len + 1] != '/') {
    return NAOS_MSG_INVALID;
  }

  // get paths
  char *from = strdup(naos_fs_concat_path((const char *)msg.data));
  const char *to = naos_fs_concat_path((const char *)&msg.data[from_len + 1]);

  // remove existing file, if any
  remove(to);

  // rename file
  int ret = rename(from, to);
  if (ret != 0) {
    free(from);
    return naos_fs_send_error(msg.session, errno);
  }

  // free from
  free(from);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_remove(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check path
  if (msg.len == 0 || msg.data[0] != '/') {
    return NAOS_MSG_INVALID;
  }

  // get path
  const char *path = naos_fs_concat_path((const char *)msg.data);

  // remove file
  int ret = remove(path);
  if (ret != 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_fs_handle_sha256(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check length
  if (msg.len == 0 || msg.data[0] != '/') {
    return NAOS_MSG_INVALID;
  }

  // get path
  const char *path = naos_fs_concat_path((const char *)msg.data);

  // open file
  int fd = open(path, O_RDONLY, 0);
  if (fd < 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // prepare context
  mbedtls_sha256_context ctx;
  mbedtls_sha256_init(&ctx);

  // start checksum
  int ret = mbedtls_sha256_starts_ret(&ctx, false);
  if (ret != 0) {
    close(fd);
    mbedtls_sha256_free(&ctx);
    return NAOS_MSG_ERROR;
  }

  // prepare data
  uint8_t data[1024];

  for (int i = 1;; i++) {
    // read data
    ssize_t len = read(fd, data, sizeof(data));
    if (len < 0) {
      close(fd);
      mbedtls_sha256_free(&ctx);
      return naos_fs_send_error(msg.session, errno);
    }

    // check for EOF
    if (len == 0) {
      break;
    }

    // update checksum
    ret = mbedtls_sha256_update_ret(&ctx, data, len);
    if (ret != 0) {
      close(fd);
      mbedtls_sha256_free(&ctx);
      return NAOS_MSG_ERROR;
    }

    // yield to system every 10th iteration
    if (i % 10 == 0) {
      naos_delay(1);
    }
  }

  // reply structure:
  // TYPE (1) | DATA (32)

  // prepare reply
  uint8_t reply[33] = {NAOS_FS_REPLY_SHA256};

  // finish checksum
  ret = mbedtls_sha256_finish_ret(&ctx, reply + 1);
  if (ret != 0) {
    close(fd);
    mbedtls_sha256_free(&ctx);
    return NAOS_MSG_ERROR;
  }

  // close file and free context
  close(fd);
  mbedtls_sha256_free(&ctx);

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_FS_ENDPOINT,
      .data = reply,
      .len = 33,
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_fs_handle(naos_msg_t msg) {
  // message structure:
  // CMD (1) | *

  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // get command
  naos_fs_cmd_t cmd = msg.data[0];

  // resize message
  msg.data = &msg.data[1];
  msg.len -= 1;

  // acquire mutex
  NAOS_LOCK(naos_fs_mutex);

  // handle command
  naos_msg_reply_t reply;
  switch (cmd) {
    case NAOS_FS_CMD_STAT:
      reply = naos_fs_handle_stat(msg);
      break;
    case NAOS_FS_CMD_LIST:
      reply = naos_fs_handle_list(msg);
      break;
    case NAOS_FS_CMD_OPEN:
      reply = naos_fs_handle_open(msg);
      break;
    case NAOS_FS_CMD_READ:
      reply = naos_fs_handle_read(msg);
      break;
    case NAOS_FS_CMD_WRITE:
      reply = naos_fs_handle_write(msg);
      break;
    case NAOS_FS_CMD_CLOSE:
      reply = naos_fs_handle_close(msg);
      break;
    case NAOS_FS_CMD_RENAME:
      reply = naos_fs_handle_rename(msg);
      break;
    case NAOS_FS_CMD_REMOVE:
      reply = naos_fs_handle_remove(msg);
      break;
    case NAOS_FS_CMD_SHA256:
      reply = naos_fs_handle_sha256(msg);
      break;
    default:
      reply = NAOS_MSG_UNKNOWN;
  }

  // acquire mutex
  NAOS_UNLOCK(naos_fs_mutex);

  return reply;
}

static void naos_fs_cleanup() {
  // acquire mutex
  NAOS_LOCK(naos_fs_mutex);

  // get time
  int64_t now = naos_millis();

  // close unused open files
  for (int i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (naos_fs_files[i].active && now - naos_fs_files[i].ts > 5000) {
      close(naos_fs_files[i].fd);
      naos_fs_files[i] = (naos_fs_file_t){0};
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_fs_mutex);
}

void naos_fs_mount_fat(const char *path, const char *label, int max_files) {
  // mount FAT filesystem
  wl_handle_t wl_handle;
  const esp_vfs_fat_mount_config_t mount_config = {
      .max_files = max_files,
      .format_if_mount_failed = true,
      .allocation_unit_size = CONFIG_WL_SECTOR_SIZE,
  };
  ESP_ERROR_CHECK(esp_vfs_fat_spiflash_mount(path, label, &mount_config, &wl_handle));
}

void naos_fs_install(naos_fs_config_t cfg) {
  // check root (must begin with "/" and not end with "/")
  if (cfg.root != NULL && (cfg.root[0] != '/' || (strlen(cfg.root) > 1 && cfg.root[strlen(cfg.root) - 1] == '/'))) {
    ESP_ERROR_CHECK(ESP_FAIL);
    return;
  }

  // create mutex
  naos_fs_mutex = naos_mutex();

  // store config
  naos_fs_config = cfg;

  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_FS_ENDPOINT,
      .name = "fs",
      .handle = naos_fs_handle,
  });

  // run cleanup periodically
  naos_repeat("fs", 1000, naos_fs_cleanup);
}
