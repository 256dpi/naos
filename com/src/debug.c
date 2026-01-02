#include <stdlib.h>
#include <string.h>
#include <esp_log.h>
#include <esp_partition.h>
#include <esp_core_dump.h>

#include <naos/debug.h>
#include <naos/msg.h>
#include <naos/sys.h>

#include "log.h"
#include "utils.h"

#define NAOS_DEBUG_ENDPOINT 0x7
#define NAOS_DEBUG_LOG_SUBS 8

typedef enum {
  NAOS_DEBUG_CDP_CHECK,
  NAOS_DEBUG_CDP_READ,
  NAOS_DEBUG_CDP_DELETE,
  NAOS_DEBUG_LOG_START,
  NAOS_DEBUG_LOG_STOP,
} naos_debug_cmd_t;

static naos_mutex_t naos_debug_mutex = 0;
static uint16_t naos_debug_log_subs[NAOS_DEBUG_LOG_SUBS] = {0};

static naos_msg_reply_t naos_debug_handle_cdp_check(naos_msg_t msg) {
  // command structure:
  // -

  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // get coredump size
  uint32_t size = naos_debug_cdp_size();

  // get coredump reason
  char buf[sizeof(size) + 256] = {0};
  naos_debug_cdp_reason(buf + sizeof(size), sizeof(buf) - sizeof(size));

  // reply structure:
  // SIZE (4) | REASON (*)

  // write size
  memcpy(buf, &size, sizeof(size));

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_DEBUG_ENDPOINT,
      .data = (uint8_t*)buf,
      .len = sizeof(size) + strlen(buf + sizeof(size)),
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_debug_handle_cdp_read(naos_msg_t msg) {
  // command structure:
  // OFFSET (4) | LENGTH (4)

  // check path
  if (msg.len != 8) {
    return NAOS_MSG_INVALID;
  }

  // get size
  uint32_t size = naos_debug_cdp_size();

  // get offset and length
  uint32_t offset;
  uint32_t length;
  memcpy(&offset, msg.data, sizeof(offset));
  memcpy(&length, &msg.data[4], sizeof(length));

  // determine length if zero or limit length
  if (length == 0) {
    if (size > offset) {
      length = size - offset;
    }
  } else if (offset + length > size) {
    length = size - offset;
  }

  // determine max chunk size
  size_t max_chunk_size = naos_msg_get_mtu(msg.session) - 16;
  if (max_chunk_size > length) {
    max_chunk_size = length;
  }

  // reply structure:
  // OFFSET (4) | DATA (*)

  // prepare data
  uint8_t* data = calloc(4 + max_chunk_size, 1);

  // read and reply with chunks
  uint32_t total = 0;
  while (total < length) {
    // determine chunk size
    size_t chunk_size = (length - total) < max_chunk_size ? (length - total) : max_chunk_size;

    // write chunk offset
    uint32_t chunk_offset = offset + total;
    memcpy(data, &chunk_offset, sizeof(chunk_offset));

    // read chunk
    naos_debug_cdp_read(chunk_offset, chunk_size, data + 4);

    // send reply
    naos_msg_send((naos_msg_t){
        .session = msg.session,
        .endpoint = NAOS_DEBUG_ENDPOINT,
        .data = data,
        .len = 4 + chunk_size,
    });

    // increment total
    total += chunk_size;

    // yield to system
    naos_delay(1);
  }

  // free data
  free(data);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_debug_handle_cdp_delete(naos_msg_t msg) {
  // command structure:
  // -

  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // delete coredump
  naos_debug_cdp_delete();

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_debug_handle_log_start(naos_msg_t msg) {
  // command structure:
  // -

  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // lock mutex
  naos_lock(naos_debug_mutex);

  // check if already subscribed
  bool subscribed = false;
  for (size_t i = 0; i < NAOS_DEBUG_LOG_SUBS; i++) {
    if (naos_debug_log_subs[i] == msg.session) {
      subscribed = true;
      break;
    }
  }

  // subscribe if not already subscribed
  if (!subscribed) {
    for (size_t i = 0; i < NAOS_DEBUG_LOG_SUBS; i++) {
      if (naos_debug_log_subs[i] == 0) {
        naos_debug_log_subs[i] = msg.session;
        subscribed = true;
        break;
      }
    }
  }

  // unlock mutex
  naos_unlock(naos_debug_mutex);

  // return error if not subscribed
  if (!subscribed) {
    return NAOS_MSG_ERROR;
  }

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_debug_handle_log_stop(naos_msg_t msg) {
  // command structure:
  // -

  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // acquire mutex
  naos_lock(naos_debug_mutex);

  // remove subscriptions
  for (size_t i = 0; i < NAOS_DEBUG_LOG_SUBS; i++) {
    if (naos_debug_log_subs[i] == msg.session) {
      naos_debug_log_subs[i] = 0;
    }
  }

  // release mutex
  naos_unlock(naos_debug_mutex);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_debug_handle(naos_msg_t msg) {
  // message structure:
  // CMD (1) | *

  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // check lock status
  if (naos_msg_is_locked(msg.session)) {
    return NAOS_MSG_LOCKED;
  }

  // get command
  naos_debug_cmd_t cmd = msg.data[0];

  // resize message
  msg.data = &msg.data[1];
  msg.len -= 1;

  // handle command
  naos_msg_reply_t reply;
  switch (cmd) {
    case NAOS_DEBUG_CDP_CHECK:
      reply = naos_debug_handle_cdp_check(msg);
      break;
    case NAOS_DEBUG_CDP_READ:
      reply = naos_debug_handle_cdp_read(msg);
      break;
    case NAOS_DEBUG_CDP_DELETE:
      reply = naos_debug_handle_cdp_delete(msg);
      break;
    case NAOS_DEBUG_LOG_START:
      reply = naos_debug_handle_log_start(msg);
      break;
    case NAOS_DEBUG_LOG_STOP:
      reply = naos_debug_handle_log_stop(msg);
      break;
    default:
      reply = NAOS_MSG_UNKNOWN;
  }

  return reply;
}

static void naos_debug_sink(const char* msg) {
  // copy subscriptions
  uint16_t subs[NAOS_DEBUG_LOG_SUBS];
  naos_lock(naos_debug_mutex);
  memcpy(subs, naos_debug_log_subs, sizeof(naos_debug_log_subs));
  naos_unlock(naos_debug_mutex);

  // send message to all subscribers
  for (size_t i = 0; i < NAOS_DEBUG_LOG_SUBS; i++) {
    if (subs[i] != 0) {
      naos_msg_send((naos_msg_t){
          .session = subs[i],
          .endpoint = NAOS_DEBUG_ENDPOINT,
          .data = (uint8_t*)msg,
          .len = strlen(msg),
      });
    }
  }
}

static void naos_debug_cleanup(uint16_t session) {
  // acquire mutex
  naos_lock(naos_debug_mutex);

  // remove subscriptions
  for (size_t i = 0; i < NAOS_DEBUG_LOG_SUBS; i++) {
    if (naos_debug_log_subs[i] == session) {
      naos_debug_log_subs[i] = 0;
    }
  }

  // release mutex
  naos_unlock(naos_debug_mutex);
}

void naos_debug_install() {
  // create mutex
  naos_debug_mutex = naos_mutex();

  // subscribe to logs
  naos_log_register(naos_debug_sink);

  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_DEBUG_ENDPOINT,
      .name = "debug",
      .handle = naos_debug_handle,
      .cleanup = naos_debug_cleanup,
  });
}

static const esp_partition_t* naos_coredump_partition() {
  // find partition if not already done
  static bool initialized = false;
  static const esp_partition_t* p = NULL;
  if (!initialized) {
    p = esp_partition_find_first(ESP_PARTITION_TYPE_DATA, ESP_PARTITION_SUBTYPE_DATA_COREDUMP, NULL);
    initialized = true;
  }

  return p;
}

uint32_t naos_debug_cdp_size() {
  // get coredump partition
  const esp_partition_t* p = naos_coredump_partition();
  if (p == NULL) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_debug_cdp_size: missing partition");
    return 0;
  }

  // read size
  uint32_t size = 0;
  ESP_ERROR_CHECK(esp_partition_read(p, 0, &size, sizeof(size)));
  if (size < sizeof(size) || size > p->size) {
    return 0;
  }

  return size;
}

bool naos_debug_cdp_reason(char* buf, size_t len) {
  // check size
  if (!naos_debug_cdp_size()) {
    return false;
  }

  // get coredump reason
  esp_err_t err = esp_core_dump_get_panic_reason(buf, len);
  if (err != ESP_OK) {
    return false;
  }

  return true;
}

void naos_debug_cdp_read(uint32_t offset, uint32_t length, void* buf) {
  // get coredump partition
  const esp_partition_t* p = naos_coredump_partition();
  if (p == NULL) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_debug_cdp_read: missing partition");
    return;
  }

  // check bounds
  if (offset > p->size || length > (p->size - offset)) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_debug_cdp_read: out of bounds");
    return;
  }

  // read coredump chunk
  ESP_ERROR_CHECK(esp_partition_read(p, offset, buf, length));
}

void naos_debug_cdp_delete() {
  // get coredump partition
  const esp_partition_t* p = naos_coredump_partition();
  if (p == NULL) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_debug_cdp_delete: missing partition");
    return;
  }

  // reset size
  uint32_t size = 0xFFFFFFFF;
  ESP_ERROR_CHECK(esp_partition_write(p, 0, &size, sizeof(size)));
}
