#include <nvs.h>
#include <stdlib.h>
#include <string.h>

#include "settings.h"

static nvs_handle naos_settings_nvs_handle;

// TODO: Rename NVS namespace in a major release.

static const char* naos_settings_key(naos_setting_t setting) {
  switch (setting) {
    case NAOS_SETTING_WIFI_SSID:
      return "wifi-ssid";
    case NAOS_SETTING_WIFI_PASSWORD:
      return "wifi-password";
    case NAOS_SETTING_MQTT_HOST:
      return "mqtt-host";
    case NAOS_SETTING_MQTT_PORT:
      return "mqtt-port";
    case NAOS_SETTING_MQTT_CLIENT_ID:
      return "mqtt-client-id";
    case NAOS_SETTING_MQTT_USERNAME:
      return "mqtt-username";
    case NAOS_SETTING_MQTT_PASSWORD:
      return "mqtt-password";
    case NAOS_SETTING_DEVICE_NAME:
      return "device-name";
    case NAOS_SETTING_BASE_TOPIC:
      return "base-topic";
    default:
      return "";
  }
}

void naos_settings_init() {
  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-ble", NVS_READWRITE, &naos_settings_nvs_handle));
}

char* naos_settings_read(naos_setting_t setting) {
  // get key for setting
  const char* key = naos_settings_key(setting);

  // get value size
  size_t required_size = 0;
  esp_err_t err = nvs_get_str(naos_settings_nvs_handle, key, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    return strdup("");
  } else {
    ESP_ERROR_CHECK(err);
  }

  // allocate value
  char* value = malloc(required_size);
  ESP_ERROR_CHECK(nvs_get_str(naos_settings_nvs_handle, key, value, &required_size));

  return value;
}

void naos_settings_write(naos_setting_t setting, const char* value) {
  // get key for setting
  const char* key = naos_settings_key(setting);

  // save value
  ESP_ERROR_CHECK(nvs_set_str(naos_settings_nvs_handle, key, value));
  ESP_ERROR_CHECK(nvs_commit(naos_settings_nvs_handle));
}
