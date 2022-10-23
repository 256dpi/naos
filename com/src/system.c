#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <nvs_flash.h>

#include "config.h"
#include "system.h"
#include "manager.h"
#include "monitor.h"
#include "naos.h"
#include "net.h"
#include "params.h"
#include "settings.h"
#include "task.h"
#include "update.h"
#include "utils.h"

#ifndef CONFIG_NAOS_BLE_DISABLE
#include "ble.h"
#endif

#ifndef CONFIG_NAOS_MQTT_DISABLE
#include "mqtt.h"
#endif

SemaphoreHandle_t naos_system_mutex;
static naos_status_t naos_system_status;

static void naos_system_set_status(naos_status_t status) {
  // get name
  const char *name = naos_status_str(status);

  // change state
  naos_system_status = status;

  // notify connection status
  naos_config_notify(NAOS_CONFIG_NOTIFICATION_DESCRIPTION);

  // call status callback if present
  if (naos_config()->status_callback != NULL) {
    naos_acquire();
    naos_config()->status_callback(status);
    naos_release();
  }

  // log new status
  ESP_LOGI(NAOS_LOG_TAG, "naos_system_set_status: %s", name);
}

static void naos_system_task() {
  for (;;) {
    // wait some time
    naos_delay(100);

    // get old status
    naos_status_t old_status = naos_system_status;

    // get net status
    naos_net_status_t net = naos_net_check();

#ifndef CONFIG_NAOS_MQTT_DISABLE
    // get mqtt status
    naos_mqtt_status_t mqtt = {0};
    mqtt = naos_mqtt_check();
#endif

    // calculate new status
    naos_status_t new_status = NAOS_DISCONNECTED;
    if (net.connected_any) {
      new_status = NAOS_CONNECTED;
    }
#ifndef CONFIG_NAOS_MQTT_DISABLE
    if (net.connected_any && mqtt.connected) {
      new_status = NAOS_NETWORKED;
    }
#endif

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

#ifndef CONFIG_NAOS_MQTT_DISABLE
    // manage mqtt
    if (mqtt.running && new_status == NAOS_DISCONNECTED) {
      // stop mqtt
      naos_mqtt_stop();

    } else if (!mqtt.running && new_status != NAOS_DISCONNECTED) {
      // start mqtt
      naos_system_configure_mqtt();
    }
#endif
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
#ifndef CONFIG_NAOS_BLE_DISABLE
  naos_ble_init();
#endif

  // initialize network stack
  naos_net_init();

  // initialize mqtt client
#ifndef CONFIG_NAOS_MQTT_DISABLE
  naos_mqtt_init(naos_manager_handle);
#endif

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

void naos_system_configure_wifi() {
  // acquire mutex
  NAOS_LOCK(naos_system_mutex);

  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_system_configure_wifi");

  // get settings
  char *wifi_ssid = naos_settings_read(NAOS_SETTING_WIFI_SSID);
  char *wifi_password = naos_settings_read(NAOS_SETTING_WIFI_PASSWORD);

  // re-configure WiFi
  naos_net_configure_wifi(wifi_ssid, wifi_password);

  // free strings
  free(wifi_ssid);
  free(wifi_password);

  // release mutex
  NAOS_UNLOCK(naos_system_mutex);
}

void naos_system_configure_mqtt() {
  // acquire mutex
  NAOS_LOCK(naos_system_mutex);

  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_system_configure_mqtt");

  // get settings
  char *mqtt_host = naos_settings_read(NAOS_SETTING_MQTT_HOST);
  char *mqtt_port = naos_settings_read(NAOS_SETTING_MQTT_PORT);
  char *mqtt_client_id = naos_settings_read(NAOS_SETTING_MQTT_CLIENT_ID);
  char *mqtt_username = naos_settings_read(NAOS_SETTING_MQTT_USERNAME);
  char *mqtt_password = naos_settings_read(NAOS_SETTING_MQTT_PASSWORD);
  char *base_topic = naos_settings_read(NAOS_SETTING_BASE_TOPIC);

#ifndef CONFIG_NAOS_MQTT_DISABLE
  // stop MQTT
  naos_mqtt_stop();

  // start MQTT
  naos_mqtt_start(mqtt_host, mqtt_port, mqtt_client_id, mqtt_username, mqtt_password, base_topic);
#endif

  // free strings
  free(mqtt_host);
  free(mqtt_port);
  free(mqtt_client_id);
  free(mqtt_username);
  free(mqtt_password);
  free(base_topic);

  // release mutex
  NAOS_UNLOCK(naos_system_mutex);
}
