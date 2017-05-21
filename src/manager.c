#include <esp_log.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <string.h>

#include "ble.h"
#include "device.h"
#include "general.h"
#include "manager.h"
#include "mqtt.h"
#include "update.h"

#define NADK_MANAGER_CHUNK_SIZE NADK_MQTT_BUFFER_SIZE - 256

#define NADK_MANAGER_HEARTBEAT_INTERVAL 5000

static SemaphoreHandle_t nadk_manager_mutex;

static TaskHandle_t nadk_manager_task;

static bool nadk_manager_process_started = false;

static void nadk_manager_send_heartbeat() {
  // get device name
  char *device_name = nadk_ble_get_string(NADK_BLE_ID_DEVICE_NAME);

  // send heartbeat
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%d,%d", nadk_device()->type, nadk_device()->version, device_name,
           esp_get_free_heap_size(), nadk_millis());
  nadk_publish_str("nadk/heartbeat", buf, 0, false, NADK_LOCAL);

  // free string
  free(device_name);
}

static void nadk_manager_send_announcement() {
  // get device name & base topic
  char *device_name = nadk_ble_get_string(NADK_BLE_ID_DEVICE_NAME);
  char *base_topic = nadk_ble_get_string(NADK_BLE_ID_BASE_TOPIC);

  // send announce
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", nadk_device()->type, nadk_device()->version, device_name, base_topic);
  nadk_publish_str("nadk/announcement", buf, 0, false, NADK_GLOBAL);

  // free strings
  free(device_name);
  free(base_topic);
}

static void nadk_manager_process(void *p) {
  for (;;) {
    // acquire mutex
    NADK_LOCK(nadk_manager_mutex);

    // send heartbeat
    nadk_manager_send_heartbeat();

    // release mutex
    NADK_UNLOCK(nadk_manager_mutex);

    // wait for next interval
    nadk_sleep(NADK_MANAGER_HEARTBEAT_INTERVAL);
  }
}

void nadk_manager_init() {
  // create mutex
  nadk_manager_mutex = xSemaphoreCreateMutex();
}

void nadk_manager_start() {
  // acquire mutex
  NADK_LOCK(nadk_manager_mutex);

  // check if already running
  if (nadk_manager_process_started) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_manager_start: already started");
    NADK_UNLOCK(nadk_manager_mutex);
    return;
  }

  // set flag
  nadk_manager_process_started = true;

  // create task
  ESP_LOGI(NADK_LOG_TAG, "nadk_manager_start: create task");
  xTaskCreatePinnedToCore(nadk_manager_process, "nadk-manager", 2048, NULL, 2, &nadk_manager_task, 1);

  // subscribe to global topics
  nadk_subscribe("nadk/collect", 0, NADK_GLOBAL);

  // subscribe to local topics
  nadk_subscribe("nadk/update/begin", 0, NADK_LOCAL);
  nadk_subscribe("nadk/update/chunk", 0, NADK_LOCAL);
  nadk_subscribe("nadk/update/finish", 0, NADK_LOCAL);

  // send initial announcement
  nadk_manager_send_announcement();

  // release mutex
  NADK_UNLOCK(nadk_manager_mutex);
}

bool nadk_manager_handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // acquire mutex
  NADK_LOCK(nadk_manager_mutex);

  // check collect
  if (scope == NADK_GLOBAL && strcmp(topic, "nadk/collect") == 0) {
    // send announcement
    nadk_manager_send_announcement();

    // release mutex
    NADK_UNLOCK(nadk_manager_mutex);

    return true;
  }

  // check update begin
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/begin") == 0) {
    // get update size
    long long int total = strtoll(payload, NULL, 10);
    ESP_LOGI(NADK_LOG_TAG, "nadk_manager_handle: begin update with size %lld", total);

    // begin update
    nadk_update_begin((uint16_t)total);

    // request first chunk
    nadk_publish_num("nadk/update/next", NADK_MANAGER_CHUNK_SIZE, 0, false, NADK_LOCAL);

    // release mutex
    NADK_UNLOCK(nadk_manager_mutex);

    return true;
  }

  // check update chunk
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/chunk") == 0) {
    // write chunk (very time expensive)
    nadk_update_write(payload, (uint16_t)len);
    ESP_LOGI(NADK_LOG_TAG, "nadk_manager_handle: wrote %d bytes chunk", len);

    // request next chunk
    nadk_publish_num("nadk/update/next", NADK_MANAGER_CHUNK_SIZE, 0, false, NADK_LOCAL);

    // release mutex
    NADK_UNLOCK(nadk_manager_mutex);

    return true;
  }

  // check update finish
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/finish") == 0) {
    // finish update
    nadk_update_finish();

    // release mutex
    NADK_UNLOCK(nadk_manager_mutex);

    return true;
  }

  // release mutex
  NADK_UNLOCK(nadk_manager_mutex);

  return false;
}

void nadk_manager_stop() {
  // acquire mutex
  NADK_LOCK(nadk_manager_mutex);

  // check if task is still running
  if (!nadk_manager_process_started) {
    NADK_UNLOCK(nadk_manager_mutex);
    return;
  }

  // set flag
  nadk_manager_process_started = false;

  // remove task
  ESP_LOGI(NADK_LOG_TAG, "nadk_manager_stop: deleting task");
  vTaskDelete(nadk_manager_task);

  // release mutex
  NADK_UNLOCK(nadk_manager_mutex);
}
