#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <nvs_flash.h>
#include <string.h>

#include "ble.h"
#include "manager.h"
#include "monitor.h"
#include "mqtt.h"
#include "naos.h"
#include "params.h"
#include "settings.h"
#include "task.h"
#include "update.h"
#include "utils.h"
#include "net.h"

SemaphoreHandle_t naos_system_mutex;

static naos_status_t naos_system_status;

static const char *naos_system_status_string(naos_status_t status) {
  switch (status) {
    case NAOS_DISCONNECTED:
      return "Disconnected";
    case NAOS_CONNECTED:
      return "Connected";
    case NAOS_NETWORKED:
      return "Networked";
    default:
      return "Unknown";
  }
}

static void naos_system_set_status(naos_status_t status) {
  // get name
  const char *name = naos_system_status_string(status);

  // change state
  naos_system_status = status;

  // update connection status
  naos_ble_notify(NAOS_BLE_CHAR_CONNECTION_STATUS, (char *)name);

  // call status callback if present
  if (naos_config()->status_callback != NULL) {
    naos_acquire();
    naos_config()->status_callback(status);
    naos_release();
  }

  ESP_LOGI(NAOS_LOG_TAG, "naos_system_set_status: %s", name);
}

static void naos_system_configure_wifi() {
  // get ssid & password
  char *wifi_ssid = naos_settings_read(NAOS_SETTING_WIFI_SSID);
  char *wifi_password = naos_settings_read(NAOS_SETTING_WIFI_PASSWORD);

  // configure Wi-Fi
  naos_net_configure_wifi(wifi_ssid, wifi_password);

  // free strings
  free(wifi_ssid);
  free(wifi_password);
}

static void naos_system_configure_mqtt() {
  // get settings
  char *mqtt_host = naos_settings_read(NAOS_SETTING_MQTT_HOST);
  char *mqtt_port = naos_settings_read(NAOS_SETTING_MQTT_PORT);
  char *mqtt_client_id = naos_settings_read(NAOS_SETTING_MQTT_CLIENT_ID);
  char *mqtt_username = naos_settings_read(NAOS_SETTING_MQTT_USERNAME);
  char *mqtt_password = naos_settings_read(NAOS_SETTING_MQTT_PASSWORD);
  char *base_topic = naos_settings_read(NAOS_SETTING_BASE_TOPIC);

  // stop MQTT
  naos_mqtt_stop();

  // start MQTT
  naos_mqtt_start(mqtt_host, mqtt_port, mqtt_client_id, mqtt_username, mqtt_password, base_topic);

  // free strings
  free(mqtt_host);
  free(mqtt_port);
  free(mqtt_client_id);
  free(mqtt_username);
  free(mqtt_password);
  free(base_topic);
}

static void naos_system_handle_command(const char *command) {
  // acquire mutex
  NAOS_LOCK(naos_system_mutex);

  // TODO: Replace "boot-factory" with "boot-alpha" and "boot-beta"?

  // detect command
  bool ping = strcmp(command, "ping") == 0;
  bool restart_mqtt = strcmp(command, "restart-mqtt") == 0;
  bool restart_wifi = strcmp(command, "restart-wifi") == 0;
  bool boot_factory = strcmp(command, "boot-factory") == 0;

  // handle ping
  if (ping) {
    // call ping callback if present
    if (naos_config()->ping_callback != NULL) {
      naos_acquire();
      naos_config()->ping_callback();
      naos_release();
    }
  }

  // handle boot factory
  else if (boot_factory) {
    // select factory partition
    ESP_ERROR_CHECK(esp_ota_set_boot_partition(
        esp_partition_find_first(ESP_PARTITION_TYPE_APP, ESP_PARTITION_SUBTYPE_APP_FACTORY, NULL)));

    // restart chip
    esp_restart();
  }

  // handle Wi-Fi restart
  else if (restart_wifi) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_system_ble_callback: restart Wi-Fi");

    // restart Wi-Fi
    naos_system_configure_wifi();
  }

  // handle mqtt restart
  else if (restart_mqtt) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_system_ble_callback: restart mqtt");

    // restart MQTT
    naos_system_configure_mqtt();
  }

  // release mutex
  NAOS_UNLOCK(naos_system_mutex);
}

static char *naos_system_read_callback(naos_ble_conn_t *conn, naos_ble_char_t ch) {
  switch (ch) {
    case NAOS_BLE_CHAR_WIFI_SSID:
      return naos_settings_read(NAOS_SETTING_WIFI_SSID);
    case NAOS_BLE_CHAR_WIFI_PASSWORD:
      return naos_settings_read(NAOS_SETTING_WIFI_PASSWORD);
    case NAOS_BLE_CHAR_MQTT_HOST:
      return naos_settings_read(NAOS_SETTING_MQTT_HOST);
    case NAOS_BLE_CHAR_MQTT_PORT:
      return naos_settings_read(NAOS_SETTING_MQTT_PORT);
    case NAOS_BLE_CHAR_MQTT_CLIENT_ID:
      return naos_settings_read(NAOS_SETTING_MQTT_CLIENT_ID);
    case NAOS_BLE_CHAR_MQTT_USERNAME:
      return naos_settings_read(NAOS_SETTING_MQTT_USERNAME);
    case NAOS_BLE_CHAR_MQTT_PASSWORD:
      return naos_settings_read(NAOS_SETTING_MQTT_PASSWORD);
    case NAOS_BLE_CHAR_DEVICE_TYPE:
      return strdup(naos_config()->device_type);
    case NAOS_BLE_CHAR_DEVICE_NAME:
      return naos_settings_read(NAOS_SETTING_DEVICE_NAME);
    case NAOS_BLE_CHAR_BASE_TOPIC:
      return naos_settings_read(NAOS_SETTING_BASE_TOPIC);
    case NAOS_BLE_CHAR_CONNECTION_STATUS:
      return strdup(naos_system_status_string(naos_system_status));
    case NAOS_BLE_CHAR_BATTERY_LEVEL:
      if (naos_config()->battery_level != NULL) {
        return strdup(naos_d2str(naos_config()->battery_level()));
      } else {
        return strdup("-1");
      }
    case NAOS_BLE_CHAR_COMMAND:
      return NULL;
    case NAOS_BLE_CHAR_PARAMS_LIST:
      return naos_params_list();
    case NAOS_BLE_CHAR_PARAMS_SELECT:
      return NULL;
    case NAOS_BLE_CHAR_PARAMS_VALUE:
      return naos_manager_read_param();
    default:
      return NULL;
  }
}

static void naos_system_write_callback(naos_ble_conn_t *conn, naos_ble_char_t ch, const char *value) {
  switch (ch) {
    case NAOS_BLE_CHAR_WIFI_SSID:
      naos_settings_write(NAOS_SETTING_WIFI_SSID, value);
      return;
    case NAOS_BLE_CHAR_WIFI_PASSWORD:
      naos_settings_write(NAOS_SETTING_WIFI_PASSWORD, value);
      return;
    case NAOS_BLE_CHAR_MQTT_HOST:
      naos_settings_write(NAOS_SETTING_MQTT_HOST, value);
      return;
    case NAOS_BLE_CHAR_MQTT_PORT:
      naos_settings_write(NAOS_SETTING_MQTT_PORT, value);
      return;
    case NAOS_BLE_CHAR_MQTT_CLIENT_ID:
      naos_settings_write(NAOS_SETTING_MQTT_CLIENT_ID, value);
      return;
    case NAOS_BLE_CHAR_MQTT_USERNAME:
      naos_settings_write(NAOS_SETTING_MQTT_USERNAME, value);
      return;
    case NAOS_BLE_CHAR_MQTT_PASSWORD:
      naos_settings_write(NAOS_SETTING_MQTT_PASSWORD, value);
      return;
    case NAOS_BLE_CHAR_DEVICE_TYPE:
      return;
    case NAOS_BLE_CHAR_DEVICE_NAME:
      naos_settings_write(NAOS_SETTING_DEVICE_NAME, value);
      return;
    case NAOS_BLE_CHAR_BASE_TOPIC:
      naos_settings_write(NAOS_SETTING_BASE_TOPIC, value);
      return;
    case NAOS_BLE_CHAR_CONNECTION_STATUS:
    case NAOS_BLE_CHAR_BATTERY_LEVEL:
      return;
    case NAOS_BLE_CHAR_COMMAND:
      naos_system_handle_command(value);
      return;
    case NAOS_BLE_CHAR_PARAMS_LIST:
      return;
    case NAOS_BLE_CHAR_PARAMS_SELECT:
      naos_manager_select_param(value);
      return;
    case NAOS_BLE_CHAR_PARAMS_VALUE:
      naos_manager_write_param(value);
      return;
    default:
      return;
  }
}

static void naos_system_task() {
  for (;;) {
    // wait some time
    naos_delay(100);

    // get old status
    naos_status_t old_status = naos_system_status;

    // get statuses
    naos_net_status_t net = naos_net_check();
    naos_mqtt_status_t mqtt = naos_mqtt_check();

    // calculate new status
    naos_status_t new_status = NAOS_DISCONNECTED;
    if (net.connected_any && mqtt.connected) {
      new_status = NAOS_NETWORKED;
    } else if (net.connected_any) {
      new_status = NAOS_CONNECTED;
    }

    // handle status change
    if (naos_system_status != new_status) {
      // set status
      naos_system_set_status(new_status);

      // handle task and manger
      if (new_status == NAOS_NETWORKED) {
        // start manager
        naos_manager_start();

        // start task
        naos_task_start();
      } else if (old_status == NAOS_NETWORKED) {
        // stop manager
        naos_manager_stop();

        // stop task
        naos_task_stop();
      }
    }

    // manage mqtt
    if (mqtt.running && new_status == NAOS_DISCONNECTED) {
      // stop mqtt
      naos_mqtt_stop();
    } else if (!mqtt.running && new_status != NAOS_DISCONNECTED) {
      // start mqtt
      naos_system_configure_mqtt();
    }
  }
}

static void naos_setup_task() {
  // run callback
  naos_config()->setup_callback();

  // delete itself
  vTaskDelete(NULL);
}

void naos_system_init() {
  // delay startup by max 5000ms if set
  if (naos_config()->delay_startup) {
    uint32_t delay = esp_random() / 858994;
    ESP_LOGI(NAOS_LOG_TAG, "naos_system_init: delay startup by %dms", delay);
    naos_delay(delay);
  }

  // create mutex
  naos_system_mutex = xSemaphoreCreateMutex();

  // initialize flash memory
  ESP_ERROR_CHECK(nvs_flash_init());

  // init monitor
  naos_monitor_init();

  // init settings
  naos_settings_init();

  // init parameters
  naos_params_init();

  // init task
  naos_task_init();

  // init manager
  naos_manager_init();

  // initialize bluetooth stack
  naos_ble_init(naos_system_read_callback, naos_system_write_callback);

  // initialize network stack
  naos_net_init();

  // initialize mqtt client
  naos_mqtt_init(naos_manager_handle);

  // initialize OTA
  naos_update_init();

  // set initial state
  naos_system_set_status(NAOS_DISCONNECTED);

  // initially configure Wi-Fi
  naos_system_configure_wifi();

  // initially configure MQTT
  naos_system_configure_mqtt();

  // run system task
  xTaskCreatePinnedToCore(naos_system_task, "naos-system", 4096, NULL, 2, NULL, 1);

  // run setup task if provided
  if (naos_config()->setup_callback) {
    xTaskCreatePinnedToCore(naos_setup_task, "naos-setup", 8192, NULL, 2, NULL, 1);
  }
}

naos_status_t naos_status() {
  // return current status
  return naos_system_status;
}
