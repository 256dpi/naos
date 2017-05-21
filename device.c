#include <esp_log.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <string.h>

#include <nadk.h>

#include "ble.h"
#include "device.h"
#include "general.h"
#include "mqtt.h"
#include "update.h"

#define NADK_OTA_CHUNK_SIZE NADK_MQTT_BUFFER_SIZE - 256

#define NADK_DEVICE_HEARTBEAT_INTERVAL 5000

static SemaphoreHandle_t nadk_device_mutex;

static nadk_device_t *nadk_device;

static TaskHandle_t nadk_device_task;

static bool nadk_device_process_started = false;

static uint32_t nadk_device_last_heartbeat = 0;

// TODO: Add offline device loop.
// The callbacks could be: offline, connected, online, disconnected.

static void nadk_device_send_heartbeat() {
  // get device name
  char *device_name = nadk_ble_get_string(NADK_BLE_ID_DEVICE_NAME);

  // save time
  nadk_device_last_heartbeat = nadk_millis();

  // send heartbeat
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%d,%d", nadk_device->type, nadk_device->version, device_name,
           esp_get_free_heap_size(), nadk_millis());
  nadk_publish_str("nadk/heartbeat", buf, 0, false, NADK_LOCAL);

  // free string
  free(device_name);
}

static void nadk_device_send_announcement() {
  // get device name & base topic
  char *device_name = nadk_ble_get_string(NADK_BLE_ID_DEVICE_NAME);
  char *base_topic = nadk_ble_get_string(NADK_BLE_ID_BASE_TOPIC);

  // send announce
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", nadk_device->type, nadk_device->version, device_name, base_topic);
  nadk_publish_str("nadk/announcement", buf, 0, false, NADK_GLOBAL);

  // free strings
  free(device_name);
  free(base_topic);
}

static void nadk_device_process(void *p) {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // subscribe to global topics
  nadk_subscribe("nadk/collect", 0, NADK_GLOBAL);

  // subscribe to device topics
  nadk_subscribe("nadk/update/begin", 0, NADK_LOCAL);
  nadk_subscribe("nadk/update/chunk", 0, NADK_LOCAL);
  nadk_subscribe("nadk/update/finish", 0, NADK_LOCAL);

  // call setup callback i present
  if (nadk_device->setup) {
    nadk_device->setup();
  }

  // send initial heartbeat
  nadk_device_send_heartbeat();

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);

  for (;;) {
    // acquire mutex
    NADK_LOCK(nadk_device_mutex);

    // send heartbeat if interval has been reached
    if (nadk_millis() - nadk_device_last_heartbeat > NADK_DEVICE_HEARTBEAT_INTERVAL) {
      nadk_device_send_heartbeat();
    }

    // call loop callback if present
    if (nadk_device->loop) {
      nadk_device->loop();
    }

    // release mutex
    NADK_UNLOCK(nadk_device_mutex);

    // yield to other processes
    vTaskDelay(1);
  }
}

void nadk_device_init(nadk_device_t *device) {
  // set device reference
  nadk_device = device;

  // create mutex
  nadk_device_mutex = xSemaphoreCreateMutex();
}

void nadk_device_start() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if already running
  if (nadk_device_process_started) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_device_start: already started");
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_process_started = true;

  // create task
  ESP_LOGI(NADK_LOG_TAG, "nadk_device_start: create task");
  xTaskCreatePinnedToCore(nadk_device_process, "core-device", 8192, NULL, 2, &nadk_device_task, 1);

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_stop() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if task is still running
  if (!nadk_device_process_started) {
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_process_started = false;

  // remove task
  ESP_LOGI(NADK_LOG_TAG, "nadk_device_stop: deleting task");
  vTaskDelete(nadk_device_task);

  // run terminate callback if present
  if (nadk_device->terminate) {
    nadk_device->terminate();
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_forward(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check collect
  if (scope == NADK_GLOBAL && strcmp(topic, "nadk/collect") == 0) {
    // send announcement
    nadk_device_send_announcement();

    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // check ota
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/begin") == 0) {
    // get update size
    long long int total = strtoll(payload, NULL, 10);
    ESP_LOGI(NADK_LOG_TAG, "nadk_device_forward: begin update with size %lld", total);

    // begin update
    nadk_ota_begin((uint16_t)total);

    // request first chunk
    nadk_publish_num("nadk/update/next", NADK_OTA_CHUNK_SIZE, 0, false, NADK_LOCAL);

    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // check ota chunk
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/chunk") == 0) {
    // forward chunk
    nadk_ota_forward(payload, (uint16_t)len);
    ESP_LOGI(NADK_LOG_TAG, "nadk_device_forward: wrote %d bytes chunk", len);

    // request next chunk
    nadk_publish_num("nadk/update/next", NADK_OTA_CHUNK_SIZE, 0, false, NADK_LOCAL);

    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // check ota chunk
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/finish") == 0) {
    // finish update
    nadk_ota_finish();

    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // call handle callback if present
  if (nadk_device->handle) {
    nadk_device->handle(topic, payload, len, scope);
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}
