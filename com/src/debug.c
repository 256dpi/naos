#include <stdlib.h>
#include <string.h>

#include <naos/msg.h>
#include <naos/sys.h>

#include "log.h"
#include "coredump.h"

#define NAOS_DEBUG_ENDPOINT 0x7
#define NAOS_DEBUG_LOG_SUBS 8

typedef enum {
  NAOS_DEBUG_CD_CHECK,
  NAOS_DEBUG_CD_READ,
  NAOS_DEBUG_CD_DELETE,
  NAOS_DEBUG_LOG_START,
  NAOS_DEBUG_LOG_STOP,
} naos_debug_cmd_t;

static naos_mutex_t naos_debug_mutex = 0;
static uint16_t naos_debug_log_subs[NAOS_DEBUG_LOG_SUBS] = {0};

static naos_msg_reply_t naos_debug_cd_check(naos_msg_t msg) {
  // command structure:
  // -

  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // get coredump size
  uint32_t size = naos_coredump_size();

  // reply structure:
  // SIZE (4)

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_DEBUG_ENDPOINT,
      .data = (uint8_t *)&size,
      .len = sizeof(size),
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_debug_cd_read(naos_msg_t msg) {
  // command structure:
  // OFFSET (4) | LENGTH (4)

  // check path
  if (msg.len != 8) {
    return NAOS_MSG_INVALID;
  }

  // get size
  uint32_t size = naos_coredump_size();

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
  uint8_t *data = calloc(4 + max_chunk_size, 1);

  // read and reply with chunks
  uint32_t total = 0;
  while (total < length) {
    // determine chunk size
    size_t chunk_size = (length - total) < max_chunk_size ? (length - total) : max_chunk_size;

    // write chunk offset
    uint32_t chunk_offset = offset + total;
    memcpy(&data[1], &chunk_offset, sizeof(chunk_offset));

    // read chunk
    naos_coredump_read(chunk_offset, chunk_size, data + 4);

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

static naos_msg_reply_t naos_debug_cd_delete(naos_msg_t msg) {
  // command structure:
  // -

  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // delete coredump
  naos_coredump_delete();

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_debug_log_start(naos_msg_t msg) {
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

static naos_msg_reply_t naos_debug_log_stop(naos_msg_t msg) {
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
    case NAOS_DEBUG_CD_CHECK:
      reply = naos_debug_cd_check(msg);
      break;
    case NAOS_DEBUG_CD_READ:
      reply = naos_debug_cd_read(msg);
      break;
    case NAOS_DEBUG_CD_DELETE:
      reply = naos_debug_cd_delete(msg);
      break;
    case NAOS_DEBUG_LOG_START:
      reply = naos_debug_log_start(msg);
      break;
    case NAOS_DEBUG_LOG_STOP:
      reply = naos_debug_log_stop(msg);
      break;
    default:
      reply = NAOS_MSG_UNKNOWN;
  }

  return reply;
}

static void naos_debug_sink(const char *msg) {
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
          .data = (uint8_t *)msg,
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
