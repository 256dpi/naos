#include <naos/update.h>
#include <naos/sys.h>
#include <naos/msg.h>

#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <string.h>

#include "utils.h"

// TODO: Remove events, use built-in replies.
// TODO: Verify update size and sequential writes.

#define NAOS_UPDATE_ENDPOINT 0x2

typedef enum {
  NAOS_UPDATE_BEGIN,
  NAOS_UPDATE_WRITE,
  NAOS_UPDATE_ABORT,
  NAOS_UPDATE_FINISH,
} naos_update_cmd_t;

typedef enum {
  NAOS_UPDATE_READY,
  NAOS_UPDATE_DONE,
} naos_update_event_t;

static naos_mutex_t naos_update_mutex;
static const esp_partition_t *naos_update_partition = NULL;
static size_t naos_update_size = 0;
static esp_ota_handle_t naos_update_handle = 0;
static uint16_t naos_update_session = 0;
static bool naos_update_block = false;

static naos_msg_reply_t naos_update_process(naos_msg_t msg) {
  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // check lock status
  if (naos_msg_is_locked(msg.session)) {
    return NAOS_MSG_LOCKED;
  }

  // get command
  naos_update_cmd_t cmd = (naos_update_cmd_t)msg.data[0];

  // adjust message
  msg.data++;
  msg.len--;

  // handle command
  switch (cmd) {
    case NAOS_UPDATE_BEGIN: {
      // command structure:
      // SIZE (4)

      // check length
      if (msg.len != 4) {
        return NAOS_MSG_INVALID;
      }

      // get size
      uint32_t size = 0;
      memcpy(&size, msg.data, 4);

      // begin update
      naos_update_begin(size);

      // set session
      naos_update_session = msg.session;

      // send reply to session
      uint8_t event = NAOS_UPDATE_READY;
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = NAOS_UPDATE_ENDPOINT,
          .data = &event,
          .len = 1,
      });

      return NAOS_MSG_OK;
    }

    case NAOS_UPDATE_WRITE: {
      // command structure:
      // ACKED (1) | DATA (*)

      // check length
      if (msg.len <= 1) {
        return NAOS_MSG_INVALID;
      }

      // check session
      if (naos_update_session != msg.session) {
        return NAOS_MSG_INVALID;
      }

      // get acked
      bool acked = msg.data[0] == 1;

      // write data
      naos_update_write(msg.data + 1, msg.len - 1);

      return acked ? NAOS_MSG_ACK : NAOS_MSG_OK;
    }

    case NAOS_UPDATE_ABORT: {
      // check length
      if (msg.len != 0) {
        return NAOS_MSG_INVALID;
      }

      // check session
      if (naos_update_session != msg.session) {
        return NAOS_MSG_INVALID;
      }

      // abort update
      naos_update_abort();

      // clear session
      naos_update_session = 0;

      return NAOS_MSG_ACK;
    }

    case NAOS_UPDATE_FINISH: {
      // check length
      if (msg.len != 0) {
        return NAOS_MSG_INVALID;
      }

      // check session
      if (naos_update_session != msg.session) {
        return NAOS_MSG_INVALID;
      }

      // finish update
      naos_update_finish();

      // clear session
      naos_update_session = 0;

      // send reply to session
      uint8_t event = NAOS_UPDATE_DONE;
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = NAOS_UPDATE_ENDPOINT,
          .data = &event,
          .len = 1,
      });

      return NAOS_MSG_OK;
    }

    default:
      return NAOS_MSG_UNKNOWN;
  }
}

static void naos_update_cleanup(uint16_t session) {
  // check if active session is lost
  if (naos_update_session == session) {
    // abort update
    naos_update_abort();

    // clear session
    naos_update_session = 0;
  }
}

void naos_update_init() {
  // create mutex
  naos_update_mutex = naos_mutex();

  // register endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_UPDATE_ENDPOINT,
      .name = "update",
      .handle = naos_update_process,
      .cleanup = naos_update_cleanup,
  });
}

bool naos_update_begin(size_t size) {
  // acquire mutex
  naos_lock(naos_update_mutex);

  // check block
  if (naos_update_block) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: blocked");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // abort a previous update and discard its result
  if (naos_update_handle != 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: aborting previous update...");
    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_ota_abort(naos_update_handle));
    naos_update_handle = 0;
  }

  // get update partition
  naos_update_partition = esp_ota_get_next_update_partition(NULL);
  if (naos_update_partition == NULL) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: no partition available");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_begin: beginning update...");

  // store size
  naos_update_size = size;

  // begin update (without full flash erase)
  ESP_ERROR_CHECK(esp_ota_begin(naos_update_partition, OTA_WITH_SEQUENTIAL_WRITES, &naos_update_handle));

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_begin: update begun!");

  // release mutex
  naos_unlock(naos_update_mutex);

  return true;
}

bool naos_update_write(const uint8_t *chunk, size_t len) {
  // acquire mutex
  naos_lock(naos_update_mutex);

  // check block
  if (naos_update_block) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_write: blocked");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // check handle
  if (naos_update_handle == 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_write: missing handle");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // write chunk
  ESP_ERROR_CHECK(esp_ota_write(naos_update_handle, chunk, len));

  // release mutex
  naos_unlock(naos_update_mutex);

  return true;
}

bool naos_update_abort() {
  // acquire mutex
  naos_lock(naos_update_mutex);

  // check block
  if (naos_update_block) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_abort: blocked");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // abort a previous update and discard its result
  if (naos_update_handle != 0) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_update_abort: aborting update...");
    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_ota_abort(naos_update_handle));
  }

  // clear state
  naos_update_partition = NULL;
  naos_update_size = 0;
  naos_update_handle = 0;

  // release mutex
  naos_unlock(naos_update_mutex);

  return true;
}

bool naos_update_finish() {
  // acquire mutex
  naos_lock(naos_update_mutex);

  // check block
  if (naos_update_block) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: blocked");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // check handle
  if (naos_update_handle == 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: missing handle");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: finishing update...");

  // end update
  ESP_ERROR_CHECK(esp_ota_end(naos_update_handle));

  // set boot partition
  ESP_ERROR_CHECK(esp_ota_set_boot_partition(naos_update_partition));

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: update finished!");

  // set block
  naos_update_block = true;

  // release mutex
  naos_unlock(naos_update_mutex);

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: rebooting in 1s...");

  // restart in one second
  naos_defer("naos-restart", 1000, esp_restart);

  return true;
}
