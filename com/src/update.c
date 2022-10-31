#include <naos_sys.h>

#include <esp_log.h>
#include <esp_ota_ops.h>

#include "naos.h"
#include "update.h"
#include "utils.h"

static naos_mutex_t naos_update_mutex;
static const esp_partition_t *naos_update_partition = NULL;
static esp_ota_handle_t naos_update_handle = 0;

void naos_update_init() {
  // create mutex
  naos_update_mutex = naos_mutex();
}

void naos_update_begin(size_t size) {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // abort a previous update and discard its result
  if (naos_update_handle != 0) {
    esp_ota_abort(naos_update_handle);
    naos_update_handle = 0;
  }

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_begin: start update");

  // get update partition
  naos_update_partition = esp_ota_get_next_update_partition(NULL);
  if (naos_update_partition != NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // begin update
  ESP_ERROR_CHECK(esp_ota_begin(naos_update_partition, size, &naos_update_handle));

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);
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

void naos_update_finish() {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // check handle
  if (naos_update_handle == 0) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_update_finish: missing handle");
    NAOS_UNLOCK(naos_update_mutex);
    return;
  }

  // end update
  ESP_ERROR_CHECK(esp_ota_end(naos_update_handle));

  // reset handle
  naos_update_handle = 0;

  // set boot partition
  ESP_ERROR_CHECK(esp_ota_set_boot_partition(naos_update_partition));

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_finish: update finished");

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);

  // restart system
  esp_restart();
}
