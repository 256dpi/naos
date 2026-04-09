#include <naos/update.h>
#include <naos/sys.h>
#include <naos/msg.h>

#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <string.h>

#include "utils.h"

#define NAOS_UPDATE_ENDPOINT 0x2

typedef enum {
  NAOS_UPDATE_BEGIN,
  NAOS_UPDATE_WRITE,
  NAOS_UPDATE_ABORT,
  NAOS_UPDATE_FINISH,
} naos_update_cmd_t;

static naos_mutex_t naos_update_mutex;
static const esp_partition_t *naos_update_partition = NULL;
static size_t naos_update_size = 0;
static size_t naos_update_written = 0;
static esp_ota_handle_t naos_update_handle = 0;
static uint16_t naos_update_session = 0;
static bool naos_update_block = false;

static void naos_update_reset(bool clear_session) {
  naos_update_partition = NULL;
  naos_update_size = 0;
  naos_update_written = 0;
  naos_update_handle = 0;
  if (clear_session) {
    naos_update_session = 0;
  }
}

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
      if (!naos_update_begin(size)) {
        return NAOS_MSG_ERROR;
      }

      // set session
      naos_update_session = msg.session;

      return NAOS_MSG_ACK;
    }

    case NAOS_UPDATE_WRITE: {
      // command structure:
      // ACKED (1) | OFFSET (4) | DATA (*)

      // check length
      if (msg.len <= 5) {
        return NAOS_MSG_INVALID;
      }

      // check session
      if (naos_update_session != msg.session) {
        return NAOS_MSG_INVALID;
      }

      // get acked
      bool acked = msg.data[0] == 1;

      // get offset
      uint32_t offset = 0;
      memcpy(&offset, msg.data + 1, 4);

      // write data
      if (!naos_update_write(offset, msg.data + 5, msg.len - 5)) {
        return NAOS_MSG_ERROR;
      }

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
      if (!naos_update_abort()) {
        return NAOS_MSG_ERROR;
      }

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
      if (!naos_update_finish()) {
        return NAOS_MSG_ERROR;
      }

      // clear session
      naos_update_session = 0;

      return NAOS_MSG_ACK;
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

  // reject empty updates
  if (size == 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: invalid size");
    naos_update_partition = NULL;
    naos_unlock(naos_update_mutex);
    return false;
  }

  // reject updates that do not fit into the target partition
  if (size > naos_update_partition->size) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: update too large (%u > %u)", (unsigned int)size,
             (unsigned int)naos_update_partition->size);
    naos_update_partition = NULL;
    naos_unlock(naos_update_mutex);
    return false;
  }

  // store size
  naos_update_size = size;
  naos_update_written = 0;

  // begin update (without full flash erase)
  esp_err_t err = esp_ota_begin(naos_update_partition, OTA_WITH_SEQUENTIAL_WRITES, &naos_update_handle);
  if (err == ESP_ERR_OTA_ROLLBACK_INVALID_STATE) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: %s", esp_err_to_name(err));
    naos_update_reset(false);
    naos_unlock(naos_update_mutex);
    return false;
  }
  ESP_ERROR_CHECK(err);

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_begin: update begun!");

  // release mutex
  naos_unlock(naos_update_mutex);

  return true;
}

bool naos_update_write(uint32_t offset, const uint8_t *chunk, size_t len) {
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

  // enforce monotonic sequential writes
  if (offset != naos_update_written) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_write: unexpected offset %u, expected %u", (unsigned int)offset,
             (unsigned int)naos_update_written);
    naos_unlock(naos_update_mutex);
    return false;
  }

  // check length against declared update size
  if (len > naos_update_size - naos_update_written) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_write: write exceeds declared update size");
    naos_unlock(naos_update_mutex);
    return false;
  }

  // write chunk
  esp_err_t err = esp_ota_write(naos_update_handle, chunk, len);
  if (err == ESP_ERR_OTA_VALIDATE_FAILED || err == ESP_ERR_INVALID_SIZE) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_write: %s", esp_err_to_name(err));
    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_ota_abort(naos_update_handle));
    naos_update_reset(true);
    naos_unlock(naos_update_mutex);
    return false;
  }
  ESP_ERROR_CHECK(err);

  // track bytes written
  naos_update_written += len;

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
  naos_update_reset(false);

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

  // verify size
  if (naos_update_written != naos_update_size) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: incomplete update");
    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_ota_abort(naos_update_handle));
    naos_update_reset(true);
    naos_unlock(naos_update_mutex);
    return false;
  }

  // end update
  esp_err_t err = esp_ota_end(naos_update_handle);
  if (err == ESP_ERR_OTA_VALIDATE_FAILED) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: esp_ota_end failed: %s", esp_err_to_name(err));
    naos_update_reset(true);
    naos_unlock(naos_update_mutex);
    return false;
  }
  ESP_ERROR_CHECK(err);
  naos_update_handle = 0;

  // set boot partition
  err = esp_ota_set_boot_partition(naos_update_partition);
  if (err == ESP_ERR_OTA_VALIDATE_FAILED) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: esp_ota_set_boot_partition failed: %s", esp_err_to_name(err));
    naos_update_reset(true);
    naos_unlock(naos_update_mutex);
    return false;
  }
  ESP_ERROR_CHECK(err);

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: update finished!");

  // set block
  naos_update_block = true;

  // clear state
  naos_update_reset(false);

  // release mutex
  naos_unlock(naos_update_mutex);

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: rebooting in 1s...");

  // restart in one second
  naos_defer("naos-restart", 1000, esp_restart);

  return true;
}
