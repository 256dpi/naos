#include <nvs.h>
#include <stdlib.h>
#include <string.h>

#include "settings.h"

// TODO: Rename NVS namespace in a major release.

static const char* naos_setting_keys[] = {
    [NAOS_SETTING_WIFI_SSID] = "wifi-ssid",           [NAOS_SETTING_WIFI_PASSWORD] = "wifi-password",
    [NAOS_SETTING_MQTT_HOST] = "mqtt-host",           [NAOS_SETTING_MQTT_PORT] = "mqtt-port",
    [NAOS_SETTING_MQTT_CLIENT_ID] = "mqtt-client-id", [NAOS_SETTING_MQTT_USERNAME] = "mqtt-username",
    [NAOS_SETTING_MQTT_PASSWORD] = "mqtt-password",   [NAOS_SETTING_DEVICE_NAME] = "device-name",
    [NAOS_SETTING_BASE_TOPIC] = "base-topic",
};

static nvs_handle naos_settings_nvs_handle;

const char* naos_setting2key(naos_setting_t setting) {
  // check setting and return key
  if (setting >= 0 && setting < NAOS_SETTING_MAX) {
    return naos_setting_keys[setting];
  } else {
    return NULL;
  }
}

naos_setting_t naos_key2setting(const char* key) {
  // find setting
  for (size_t i = 0; i < NAOS_SETTING_MAX; i++) {
    if (strcmp(naos_setting_keys[i], key) == 0) {
      return i;
    }
  }

  return NAOS_SETTING_UNKNOWN;
}

void naos_settings_init() {
  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-ble", NVS_READWRITE, &naos_settings_nvs_handle));
}

char* naos_settings_read(naos_setting_t setting) {
  // get key for setting
  const char* key = naos_setting2key(setting);

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
  const char* key = naos_setting2key(setting);

  // save value
  ESP_ERROR_CHECK(nvs_set_str(naos_settings_nvs_handle, key, value));
  ESP_ERROR_CHECK(nvs_commit(naos_settings_nvs_handle));
}
