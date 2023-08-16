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
} naos_fs_cmd_t;

typedef enum {
  NAOS_FS_REPLY_ERROR,
  NAOS_FS_REPLY_INFO,
  NAOS_FS_REPLY_CHUNK,
} naos_fs_reply_t;

typedef enum {
  NAOS_FS_FLAG_CREATE = 1 << 0,
  NAOS_FS_FLAG_APPEND = 1 << 1,
  NAOS_FS_FLAG_TRUNCATE = 1 << 2,
  NAOS_FS_FLAG_EXCLUSIVE = 1 << 3,
} naos_fs_flags_t;

typedef struct {
  bool active;
  int fd;
  uint16_t sid;
  int64_t ts;
  naos_fs_flags_t flags;
} naos_fs_file_t;

static naos_mutex_t naos_fs_mutex = 0;
static naos_fs_file_t naos_fs_files[NAOS_FS_MAX_FILES] = {0};

static naos_msg_err_t naos_fs_send_error(uint16_t session, int error) {
  // reply structure:
  // TYPE (1) | ERRNO (1)

  // prepare data
  uint8_t data[] = {NAOS_FS_REPLY_ERROR, error};

  // send error
  naos_msg_endpoint_send((naos_msg_t){
      .session = session,
      .endpoint = NAOS_FS_ENDPOINT,
      .data = data,
      .len = 2,
  });

  return NAOS_MSG_OK;
}

static naos_msg_err_t naos_fs_handle_stat(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check path
  if (msg.len == 0) {
    return NAOS_MSG_INCOMPLETE;
  }

  // stat path
  struct stat info;
  int ret = stat((const char *)msg.data, &info);
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
  naos_msg_endpoint_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_FS_ENDPOINT,
      .data = data,
      .len = 6,
  });

  return NAOS_MSG_OK;
}

static naos_msg_err_t naos_fs_handle_list(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check path
  if (msg.len == 0) {
    return NAOS_MSG_INCOMPLETE;
  }

  // open directory
  DIR *dir = opendir((const char *)msg.data);
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
    snprintf(path, sizeof(path), "%s/%s", msg.data, entry->d_name);

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
    naos_msg_endpoint_send((naos_msg_t){
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

static naos_msg_err_t naos_fs_handle_open(naos_msg_t msg) {
  // command structure:
  // FLAGS (1) | PATH (*)

  // check path
  if (msg.len <= 1) {
    return NAOS_MSG_INCOMPLETE;
  }

  // find free file
  naos_fs_file_t *file = NULL;
  for (size_t i = 0; i < NAOS_FS_MAX_FILES; i++) {
    if (naos_fs_files[i].ts == 0) {
      file = &naos_fs_files[i];
      break;
    }
  }
  if (file == NULL) {
    return naos_fs_send_error(msg.session, ENFILE);
  }

  // get flags
  naos_fs_flags_t flags = msg.data[0];

  // prepare open flags
  int open_flags = O_RDWR;
  if (flags & NAOS_FS_FLAG_CREATE) {
    open_flags |= O_CREAT;
  }
  if (flags & NAOS_FS_FLAG_APPEND) {
    open_flags |= O_APPEND;
  }
  if (flags & NAOS_FS_FLAG_TRUNCATE) {
    open_flags |= O_TRUNC;
  }
  if (flags & NAOS_FS_FLAG_EXCLUSIVE) {
    open_flags |= O_EXCL;
  }

  // create file
  int fd = open((const char *)(msg.data + 1), open_flags, 0644);
  if (fd < 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // set file
  file->active = true;
  file->fd = fd;
  file->sid = msg.session;
  file->ts = naos_millis();
  file->flags = flags;

  return NAOS_MSG_ACK;
}

static naos_msg_err_t naos_fs_handle_read(naos_msg_t msg) {
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

  // determine length if zero
  if (length == 0) {
    struct stat info;
    int ret = fstat(file->fd, &info);
    if (ret != 0) {
      return naos_fs_send_error(msg.session, errno);
    }
    if (info.st_size > offset) {
      length = info.st_size - offset;
    }
  }

  // seek to offset
  off_t ret = lseek(file->fd, offset, SEEK_SET);
  if (ret < 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // determine max chunk size
  size_t max_chunk_size = naos_msg_session_mtu(msg.session) - 16;

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
    naos_msg_endpoint_send((naos_msg_t){
        .session = msg.session,
        .endpoint = NAOS_FS_ENDPOINT,
        .data = data,
        .len = 5 + ret,
    });

    // increment total
    total += ret;

    // yield to system
    NAOS_UNLOCK(naos_fs_mutex);
    naos_delay(1);
    NAOS_LOCK(naos_fs_mutex);
  }

  // free data
  free(data);

  return NAOS_MSG_ACK;
}

static naos_msg_err_t naos_fs_handle_write(naos_msg_t msg) {
  // command structure:
  // OFFSET (4) | DATA (*)

  // check path
  if (msg.len <= 4) {
    return NAOS_MSG_INCOMPLETE;
  }

  // get offset
  uint32_t offset;
  memcpy(&offset, msg.data, sizeof(offset));

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

  // seek offset
  off_t ret = lseek(file->fd, offset, SEEK_SET);
  if (ret < 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  // write all data
  size_t total = 0;
  while (total < msg.len - 4) {
    ret = write(file->fd, msg.data + 4 + total, msg.len - 4 - total);
    if (ret < 0) {
      return naos_fs_send_error(msg.session, errno);
    }
    total += ret;
  }

  // update timestamp
  file->ts = naos_millis();

  return NAOS_MSG_ACK;
}

static naos_msg_err_t naos_fs_handle_close(naos_msg_t msg) {
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

static naos_msg_err_t naos_fs_handle_rename(naos_msg_t msg) {
  // command structure:
  // FROM (*) | 0 | TO (*)

  // get lengths
  size_t from_len = strlen((const char *)msg.data);
  size_t to_len = strlen((const char *)&msg.data[from_len + 1]);
  if (from_len == 0 || to_len == 0) {
    return NAOS_MSG_INCOMPLETE;
  } else if (from_len + 1 + to_len > msg.len) {
    return NAOS_MSG_INVALID;
  }

  // get strings
  const char *from = (const char *)msg.data;
  const char *to = (const char *)&msg.data[from_len + 1];

  // rename file
  int ret = rename(from, to);
  if (ret != 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  return NAOS_MSG_ACK;
}

static naos_msg_err_t naos_fs_handle_remove(naos_msg_t msg) {
  // command structure:
  // PATH (*)

  // check path
  if (msg.len == 0) {
    return NAOS_MSG_INCOMPLETE;
  }

  // get path
  const char *path = (const char *)msg.data;

  // remove file
  int ret = remove(path);
  if (ret != 0) {
    return naos_fs_send_error(msg.session, errno);
  }

  return NAOS_MSG_ACK;
}

static naos_msg_err_t naos_fs_handle(naos_msg_t msg) {
  // message structure:
  // CMD (1) | *

  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INCOMPLETE;
  }

  // get command
  naos_fs_cmd_t cmd = msg.data[0];

  // resize message
  msg.data = &msg.data[1];
  msg.len -= 1;

  // acquire mutex
  NAOS_LOCK(naos_fs_mutex);

  // handle command
  naos_msg_err_t err;
  switch (cmd) {
    case NAOS_FS_CMD_STAT:
      err = naos_fs_handle_stat(msg);
      break;
    case NAOS_FS_CMD_LIST:
      err = naos_fs_handle_list(msg);
      break;
    case NAOS_FS_CMD_OPEN:
      err = naos_fs_handle_open(msg);
      break;
    case NAOS_FS_CMD_READ:
      err = naos_fs_handle_read(msg);
      break;
    case NAOS_FS_CMD_WRITE:
      err = naos_fs_handle_write(msg);
      break;
    case NAOS_FS_CMD_CLOSE:
      err = naos_fs_handle_close(msg);
      break;
    case NAOS_FS_CMD_RENAME:
      err = naos_fs_handle_rename(msg);
      break;
    case NAOS_FS_CMD_REMOVE:
      err = naos_fs_handle_remove(msg);
      break;
    default:
      err = NAOS_MSG_UNKNOWN;
  }

  // acquire mutex
  NAOS_UNLOCK(naos_fs_mutex);

  return err;
}

static void naos_fs_cleanup() {
  // acquire mutex
  NAOS_LOCK(naos_fs_mutex);

  // get time
  int64_t now = naos_millis();

  // close left open files
  for (int i = 0; i < NAOS_FS_MAX_FILES; i++) {
    // skip inactive files
    if (!naos_fs_files[i].active) {
      continue;
    }

    // clear if unused for 5 seconds
    if (now - naos_fs_files[i].ts > 5000) {
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

void naos_fs_install() {
  // create mutex
  naos_fs_mutex = naos_mutex();

  // register endpoint
  naos_msg_endpoint_register((naos_msg_endpoint_t){
      .ref = NAOS_FS_ENDPOINT,
      .name = "fs",
      .handle = naos_fs_handle,
  });

  // run cleanup periodically
  naos_repeat("fs", 1000, naos_fs_cleanup);
}
