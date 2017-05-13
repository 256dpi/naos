#include <esp_err.h>
#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>

#include "general.h"
#include "ota.h"

static SemaphoreHandle_t nadk_ota_mutex;

static const esp_partition_t *nadk_ota_partition = NULL;
static esp_ota_handle_t nadk_ota_handle = 0;

void nadk_ota_init() {
  // create mutex
  nadk_ota_mutex = xSemaphoreCreateMutex();
}

void nadk_ota_begin(uint16_t size) {
  // acquire mutex
  NADK_LOCK(nadk_ota_mutex);

  // check handle
  if (nadk_ota_handle != 0) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_ota_begin: leftover handle");
    return;
  }

  // log message
  ESP_LOGI(NADK_LOG_TAG, "nadk_ota_begin: start update");

  // get update partition
  nadk_ota_partition = esp_ota_get_next_update_partition(NULL);
  assert(nadk_ota_partition != NULL);

  // begin update
  ESP_ERROR_CHECK(esp_ota_begin(nadk_ota_partition, size, &nadk_ota_handle));

  // release mutex
  NADK_UNLOCK(nadk_ota_mutex);
}

void nadk_ota_forward(const char *chunk, uint16_t len) {
  // acquire mutex
  NADK_LOCK(nadk_ota_mutex);

  // check handle
  if (nadk_ota_handle == 0) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_ota_forward: missing handle");
    return;
  }

  // write chunk
  ESP_ERROR_CHECK(esp_ota_write(nadk_ota_handle, (const void *)chunk, len));

  // release mutex
  NADK_UNLOCK(nadk_ota_mutex);
}

void nadk_ota_finish() {
  // acquire mutex
  NADK_LOCK(nadk_ota_mutex);

  // check handle
  if (nadk_ota_handle == 0) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_ota_finish: missing handle");
    return;
  }

  // TODO: Check if all data has been received.

  // end update
  ESP_ERROR_CHECK(esp_ota_end(nadk_ota_handle));

  // reset handle
  nadk_ota_handle = 0;

  // set boot partition
  ESP_ERROR_CHECK(esp_ota_set_boot_partition(nadk_ota_partition));

  // log message
  ESP_LOGI(NADK_LOG_TAG, "nadk_ota_finish: update finished");

  // release mutex
  NADK_UNLOCK(nadk_ota_mutex);

  // TODO: Give other components time to shut down?

  // restart system
  esp_restart();
}
