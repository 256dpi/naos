#include <naos/sys.h>
#include <naos/msg.h>

#include <string.h>
#include <esp_ota_ops.h>
#include <esp_task_wdt.h>

#include "update.h"
#include "utils.h"

#define NAOS_UPDATE_ENDPOINT 0x02

typedef enum {
  NAOS_UPDATE_BEGIN,
  NAOS_UPDATE_WRITE,
  NAOS_UPDATE_ABORT,
  NAOS_UPDATE_FINISH,
} naos_update_cmd_t;

static naos_mutex_t naos_update_mutex;
static naos_update_callback_t naos_update_callback = NULL;
static const esp_partition_t *naos_update_partition = NULL;
static size_t naos_update_size = 0;
static esp_ota_handle_t naos_update_handle = 0;
static uint16_t naos_update_session = 0;

static void naos_update_begin_task() {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // esp_ota_begin will erase flash, which may take up to 10s for 2MB,
  // we conservatively increase the task WDT timeout to 30s if enabled

  // increase task WDT timeout if enabled
#ifdef CONFIG_ESP_TASK_WDT_PANIC
  ESP_ERROR_CHECK(esp_task_wdt_init(30, true));
#elif CONFIG_ESP_TASK_WDT
  ESP_ERROR_CHECK(esp_task_wdt_init(30, false));
#endif

  // begin update
  ESP_ERROR_CHECK(esp_ota_begin(naos_update_partition, naos_update_size, &naos_update_handle));

  // restore original task WDT timeout if enabled
#ifdef CONFIG_ESP_TASK_WDT_PANIC
  ESP_ERROR_CHECK(esp_task_wdt_init(CONFIG_ESP_TASK_WDT_TIMEOUT_S, true));
#elif CONFIG_ESP_TASK_WDT
  ESP_ERROR_CHECK(esp_task_wdt_init(CONFIG_ESP_TASK_WDT_TIMEOUT_S, false));
#endif

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);

  // call callback if available
  if (naos_update_callback != NULL) {
    naos_update_callback(NAOS_UPDATE_READY);
  }

  // send reply to session
  if (naos_update_session != 0) {
    uint8_t event = NAOS_UPDATE_READY;
    naos_msg_send((naos_msg_t){
      .session = naos_update_session,
      .endpoint = NAOS_UPDATE_ENDPOINT,
      .data = &event,
      .len = 1,
    });
  }
}

static void naos_update_finish_task() {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // end update
  ESP_ERROR_CHECK(esp_ota_end(naos_update_handle));

  // set boot partition
  ESP_ERROR_CHECK(esp_ota_set_boot_partition(naos_update_partition));

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: update finished");

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);

  // call callback if available
  if (naos_update_callback != NULL) {
    naos_update_callback(NAOS_UPDATE_DONE);
  }

  // send reply to session
  if (naos_update_session != 0) {
    uint8_t event = NAOS_UPDATE_DONE;
    naos_msg_send((naos_msg_t){
        .session = naos_update_session,
        .endpoint = NAOS_UPDATE_ENDPOINT,
        .data = &event,
        .len = 1,
    });
  }

  // restart system
  esp_restart();
}

static naos_msg_reply_t naos_update_process(naos_msg_t msg) {
  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // get command
  naos_update_cmd_t cmd = (naos_update_cmd_t)msg.data[0];

  // adjust message
  msg.data++;
  msg.len--;

  // handle command
  switch (cmd) {
    case NAOS_UPDATE_BEGIN:
      // command structure:
      // SIZE (4)

      // check length
      if (msg.len != 4) {
        return NAOS_MSG_INVALID;
      }

      // get size
      uint32_t size = 0;
      memcpy(&size, msg.data, 4);

      // set session
      naos_update_session = msg.session;

      // begin update
      naos_update_begin(size, NULL);

      return NAOS_MSG_OK;

    case NAOS_UPDATE_WRITE:
      // command structure:
      // DATA (*)

      // check length
      if (msg.len == 0) {
        return NAOS_MSG_INVALID;
      }

      // check session
      if (naos_update_session != msg.session) {
        return NAOS_MSG_INVALID;
      }

      // write data
      naos_update_write(msg.data, msg.len);

      return NAOS_MSG_ACK;

    case NAOS_UPDATE_ABORT:
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

      return NAOS_MSG_ACK;

    case NAOS_UPDATE_FINISH:
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

      return NAOS_MSG_OK;

    default:
      return NAOS_MSG_UNKNOWN;
  }
}

static void naos_update_cleanup(uint16_t ref) {
  // check sessions
  if (naos_update_session == ref) {
    // abort update
    naos_update_abort();
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

void naos_update_begin(size_t size, naos_update_callback_t cb) {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // abort a previous update and discard its result
  if (naos_update_handle != 0) {
    esp_ota_abort(naos_update_handle);
    naos_update_handle = 0;
  }

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_begin: start update");

  // store callback
  naos_update_callback = cb;

  // get update partition
  naos_update_partition = esp_ota_get_next_update_partition(NULL);
  if (naos_update_partition == NULL) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_begin: no partition available");
    NAOS_UNLOCK(naos_update_mutex);
    return;
  }

  // store size
  naos_update_size = size;

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);

  // run begin task
  naos_run("update-begin", 4096, 1, naos_update_begin_task);
}

void naos_update_write(const uint8_t *chunk, size_t len) {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // check handle
  if (naos_update_handle == 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_write: missing handle");
    NAOS_UNLOCK(naos_update_mutex);
    return;
  }

  // write chunk
  ESP_ERROR_CHECK(esp_ota_write(naos_update_handle, (const void *)chunk, len));

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);
}

void naos_update_abort() {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // abort a previous update and discard its result
  if (naos_update_handle != 0) {
    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_ota_abort(naos_update_handle));
  }

  // clear state
  naos_update_callback = NULL;
  naos_update_partition = NULL;
  naos_update_size = 0;
  naos_update_handle = 0;
  naos_update_session = 0;

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);
}

void naos_update_finish() {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // check handle
  if (naos_update_handle == 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: missing handle");
    NAOS_UNLOCK(naos_update_mutex);
    return;
  }

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);

  // run finish task
  naos_run("update-finish", 4096, 1, naos_update_finish_task);
}
