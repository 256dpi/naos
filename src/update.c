#include <esp_err.h>
#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>

#include "update.h"
#include "utils.h"

static SemaphoreHandle_t naos_update_mutex;

static const esp_partition_t *naos_update_partition = NULL;
static esp_ota_handle_t naos_update_handle = 0;

void naos_update_init() {
  // create mutex
  naos_update_mutex = xSemaphoreCreateMutex();
}

void naos_update_begin(uint16_t size) {
  // acquire mutex
  NAOS_LOCK(naos_update_mutex);

  // end a previous update and discard its result
  if (naos_update_handle != 0) {
    esp_ota_end(naos_update_handle);
    naos_update_handle = 0;
  }

  // log message
  ESP_LOGI(NAOS_LOG_TAG, "naos_update_begin: start update");

  // get update partition
  naos_update_partition = esp_ota_get_next_update_partition(NULL);
  assert(naos_update_partition != NULL);

  // begin update
  ESP_ERROR_CHECK(esp_ota_begin(naos_update_partition, 0, &naos_update_handle));

  // release mutex
  NAOS_UNLOCK(naos_update_mutex);
}

void naos_update_write(uint8_t *chunk, uint16_t len) {
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
