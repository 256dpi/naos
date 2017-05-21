#include <esp_log.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <string.h>

#include <nadk.h>

#include "ble.h"
#include "device.h"
#include "general.h"
#include "led.h"
#include "mqtt.h"
#include "update.h"
#include "wifi.h"

SemaphoreHandle_t nadk_system_mutex;

typedef enum {
  NADK_SYSTEM_STATE_DISCONNECTED,
  NADK_SYSTEM_STATE_CONNECTED,
  NADK_SYSTEM_STATE_NETWORKED
} nadk_system_state_t;

static nadk_system_state_t nadk_system_current_state;

static void nadk_system_set_state(nadk_system_state_t new_state) {
  // default state name
  const char *name = "Unknown";

  // triage state
  switch (new_state) {
    // handle disconnected state
    case NADK_SYSTEM_STATE_DISCONNECTED: {
      name = "Disconnected";
      nadk_led_set(false, false);
      break;
    }

    // handle connected state
    case NADK_SYSTEM_STATE_CONNECTED: {
      name = "Connected";
      nadk_led_set(true, false);
      break;
    }

    // handle networked state
    case NADK_SYSTEM_STATE_NETWORKED: {
      name = "Networked";
      nadk_led_set(false, true);
      break;
    }
  }

  // change state
  nadk_system_current_state = new_state;

  // update connection status
  nadk_ble_set_string(NADK_BLE_ID_CONNECTION_STATUS, (char *)name);

  ESP_LOGI(NADK_LOG_TAG, "nadk_system_set_state: %s", name)
}

static void nadk_system_configure_wifi() {
  // get ssid & password
  char *wifi_ssid = nadk_ble_get_string(NADK_BLE_ID_WIFI_SSID);
  char *wifi_password = nadk_ble_get_string(NADK_BLE_ID_WIFI_PASSWORD);

  // configure wifi
  nadk_wifi_configure(wifi_ssid, wifi_password);

  // free strings
  free(wifi_ssid);
  free(wifi_password);
}

static void nadk_system_start_mqtt() {
  // get settings
  char *mqtt_host = nadk_ble_get_string(NADK_BLE_ID_MQTT_HOST);
  char *mqtt_client_id = nadk_ble_get_string(NADK_BLE_ID_MQTT_CLIENT_ID);
  char *mqtt_username = nadk_ble_get_string(NADK_BLE_ID_MQTT_USERNAME);
  char *mqtt_password = nadk_ble_get_string(NADK_BLE_ID_MQTT_PASSWORD);
  char *base_topic = nadk_ble_get_string(NADK_BLE_ID_BASE_TOPIC);

  // start mqtt
  nadk_mqtt_start(mqtt_host, 1883, mqtt_client_id, mqtt_username, mqtt_password, base_topic);

  // free strings
  free(mqtt_host);
  free(mqtt_client_id);
  free(mqtt_username);
  free(mqtt_password);
  free(base_topic);
}

static void nadk_system_ble_callback(nadk_ble_id_t id) {
  // dismiss any other changed characteristic
  if (id != NADK_BLE_ID_COMMAND) {
    return;
  }

  // acquire mutex
  NADK_LOCK(nadk_system_mutex);

  // get value
  char *value = nadk_ble_get_string(NADK_BLE_ID_COMMAND);

  // detect command
  bool restart_mqtt = strcmp(value, "restart-mqtt") == 0;
  bool restart_wifi = strcmp(value, "restart-wifi") == 0;

  // free string
  free(value);

  // handle wifi restart
  if (restart_wifi) {
    ESP_LOGI(NADK_LOG_TAG, "nadk_system_ble_callback: restart wifi");

    switch (nadk_system_current_state) {
      case NADK_SYSTEM_STATE_NETWORKED: {
        // stop device
        nadk_device_stop();

        // fallthrough
      }

      case NADK_SYSTEM_STATE_CONNECTED: {
        // stop mqtt client
        nadk_mqtt_stop();

        // change state
        nadk_system_set_state(NADK_SYSTEM_STATE_DISCONNECTED);

        // fallthrough
      }

      case NADK_SYSTEM_STATE_DISCONNECTED: {
        // restart wifi
        nadk_system_configure_wifi();
      }
    }
  }

  // handle mqtt restart
  if (restart_mqtt) {
    ESP_LOGI(NADK_LOG_TAG, "nadk_system_ble_callback: restart mqtt");

    switch (nadk_system_current_state) {
      case NADK_SYSTEM_STATE_NETWORKED: {
        // stop device
        nadk_device_stop();

        // change state
        nadk_system_set_state(NADK_SYSTEM_STATE_CONNECTED);

        // fallthrough
      }

      case NADK_SYSTEM_STATE_CONNECTED: {
        // stop mqtt client
        nadk_mqtt_stop();

        // restart mqtt
        nadk_system_start_mqtt();

        // fallthrough
      }

      case NADK_SYSTEM_STATE_DISCONNECTED: {
        // do nothing if not yet connected
      }
    }
  }

  // release mutex
  NADK_UNLOCK(nadk_system_mutex);
}

static void nadk_system_wifi_callback(nadk_wifi_status_t status) {
  // acquire mutex
  NADK_LOCK(nadk_system_mutex);

  switch (status) {
    case NADK_WIFI_STATUS_CONNECTED: {
      ESP_LOGI(NADK_LOG_TAG, "nadk_system_wifi_callback: connected");

      // check if connection is new
      if (nadk_system_current_state == NADK_SYSTEM_STATE_DISCONNECTED) {
        // change sate
        nadk_system_set_state(NADK_SYSTEM_STATE_CONNECTED);

        // start wifi
        nadk_system_start_mqtt();
      }

      break;
    }

    case NADK_WIFI_STATUS_DISCONNECTED: {
      ESP_LOGI(NADK_LOG_TAG, "nadk_system_wifi_callback: disconnected");

      // check if disconnection is new
      if (nadk_system_current_state >= NADK_SYSTEM_STATE_CONNECTED) {
        // stop mqtt
        nadk_mqtt_stop();

        // change state
        nadk_system_set_state(NADK_SYSTEM_STATE_DISCONNECTED);
      }

      break;
    }
  }

  // release mutex
  NADK_UNLOCK(nadk_system_mutex);
}

static void nadk_system_mqtt_callback(esp_mqtt_status_t status) {
  // acquire mutex
  NADK_LOCK(nadk_system_mutex);

  switch (status) {
    case ESP_MQTT_STATUS_CONNECTED: {
      ESP_LOGI(NADK_LOG_TAG, "nadk_system_mqtt_callback: connected");

      // check if connection is new
      if (nadk_system_current_state == NADK_SYSTEM_STATE_CONNECTED) {
        // change state
        nadk_system_set_state(NADK_SYSTEM_STATE_NETWORKED);

        // start device
        nadk_device_start();
      }

      break;
    }

    case ESP_MQTT_STATUS_DISCONNECTED: {
      ESP_LOGI(NADK_LOG_TAG, "nadk_system_mqtt_callback: disconnected");

      // change state
      nadk_system_set_state(NADK_SYSTEM_STATE_CONNECTED);

      // stop device
      nadk_device_stop();

      // restart mqtt
      nadk_system_start_mqtt();

      break;
    }
  }

  // release mutex
  NADK_UNLOCK(nadk_system_mutex);
}

void nadk_system_init(nadk_device_t *device) {
  // delay startup by max 5000ms
  int delay = esp_random() / 858994;
  ESP_LOGI(NADK_LOG_TAG, "nadk_system_init: delay startup by %dms", delay);
  nadk_sleep(delay);

  // create mutex
  nadk_system_mutex = xSemaphoreCreateMutex();

  // save device
  nadk_device_init(device);

  // initialize LEDs
  nadk_led_init();

  // initialize bluetooth stack
  nadk_ble_init(nadk_system_ble_callback, device->type);

  // initialize wifi stack
  nadk_wifi_init(nadk_system_wifi_callback);

  // initialize mqtt client
  nadk_mqtt_init(nadk_system_mqtt_callback, nadk_device_forward);

  // initialize OTA
  nadk_update_init();

  // set initial state
  nadk_system_set_state(NADK_SYSTEM_STATE_DISCONNECTED);

  // initially configure wifi
  nadk_system_configure_wifi();
}
