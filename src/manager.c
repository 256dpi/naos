#include <esp_log.h>
#include <esp_system.h>
#include <stdlib.h>
#include <string.h>

#include "ble.h"
#include "device.h"
#include "general.h"
#include "manager.h"
#include "mqtt.h"
#include "update.h"

#define NADK_UPDATE_CHUNK_SIZE NADK_MQTT_BUFFER_SIZE - 256

#define NADK_DEVICE_HEARTBEAT_INTERVAL 5000

static uint32_t nadk_manager_last_heartbeat = 0;

static void nadk_manager_send_heartbeat() {
  // get device name
  char *device_name = nadk_ble_get_string(NADK_BLE_ID_DEVICE_NAME);

  // save time
  nadk_manager_last_heartbeat = nadk_millis();

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

// TODO: Run process when online.
static void nadk_manager_process() {
  // send heartbeat if interval has been reached
  if (nadk_millis() - nadk_manager_last_heartbeat > NADK_DEVICE_HEARTBEAT_INTERVAL) {
    nadk_manager_send_heartbeat();
  }
}

void nadk_manager_setup() {
  // subscribe to global topics
  nadk_subscribe("nadk/collect", 0, NADK_GLOBAL);

  // subscribe to device topics
  nadk_subscribe("nadk/update/begin", 0, NADK_LOCAL);
  nadk_subscribe("nadk/update/chunk", 0, NADK_LOCAL);
  nadk_subscribe("nadk/update/finish", 0, NADK_LOCAL);

  // send initial announcement
  nadk_manager_send_announcement();

  // send initial heartbeat
  nadk_manager_send_heartbeat();
}

bool nadk_manager_handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // check collect
  if (scope == NADK_GLOBAL && strcmp(topic, "nadk/collect") == 0) {
    // send announcement
    nadk_manager_send_announcement();

    return true;
  }

  // check update begin
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/begin") == 0) {
    // get update size
    long long int total = strtoll(payload, NULL, 10);
    ESP_LOGI(NADK_LOG_TAG, "nadk_device_forward: begin update with size %lld", total);

    // begin update
    nadk_update_begin((uint16_t)total);

    // request first chunk
    nadk_publish_num("nadk/update/next", NADK_UPDATE_CHUNK_SIZE, 0, false, NADK_LOCAL);

    return true;
  }

  // check update chunk
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/chunk") == 0) {
    // forward chunk
    nadk_update_write(payload, (uint16_t)len);
    ESP_LOGI(NADK_LOG_TAG, "nadk_device_forward: wrote %d bytes chunk", len);

    // request next chunk
    nadk_publish_num("nadk/update/next", NADK_UPDATE_CHUNK_SIZE, 0, false, NADK_LOCAL);

    return true;
  }

  // check update finish
  if (scope == NADK_LOCAL && strcmp(topic, "nadk/update/finish") == 0) {
    // finish update
    nadk_update_finish();

    return true;
  }

  return false;
}

void nadk_manager_terminate() {}
