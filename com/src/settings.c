#include <nvs.h>
#include <stdlib.h>
#include <string.h>

#include "settings.h"

static const char* naos_setting_keys[] = {
    [NAOS_SETTING_WIFI_SSID] = "wifi-ssid",           [NAOS_SETTING_WIFI_PASSWORD] = "wifi-password",
    [NAOS_SETTING_MQTT_HOST] = "mqtt-host",           [NAOS_SETTING_MQTT_PORT] = "mqtt-port",
    [NAOS_SETTING_MQTT_CLIENT_ID] = "mqtt-client-id", [NAOS_SETTING_MQTT_USERNAME] = "mqtt-username",
    [NAOS_SETTING_MQTT_PASSWORD] = "mqtt-password",   [NAOS_SETTING_DEVICE_NAME] = "device-name",
    [NAOS_SETTING_BASE_TOPIC] = "base-topic",
};

static nvs_handle naos_settings_nvs_handle;

const char* naos_setting_to_key(naos_setting_t setting) {
  // check setting and return key
  if (setting >= 0 && setting < NAOS_SETTING_MAX) {
    return naos_setting_keys[setting];
  } else {
    return NULL;
  }
}

naos_setting_t naos_setting_from_key(const char* key) {
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
  ESP_ERROR_CHECK(nvs_open("naos-sys", NVS_READWRITE, &naos_settings_nvs_handle));
}

char* naos_settings_read(naos_setting_t setting) {
  // check setting
  if (setting < 0 || setting >= NAOS_SETTING_MAX) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // get key for setting
  const char* key = naos_setting_to_key(setting);

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
  // check setting
  if (setting < 0 || setting >= NAOS_SETTING_MAX) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // get key for setting
  const char* key = naos_setting_to_key(setting);

  // save value
  ESP_ERROR_CHECK(nvs_set_str(naos_settings_nvs_handle, key, value));
  ESP_ERROR_CHECK(nvs_commit(naos_settings_nvs_handle));
}

char* naos_settings_list() {
  // determine list length
  size_t length = 0;
  for (int i = 0; i < NAOS_SETTING_MAX; i++) {
    length += strlen(naos_setting_keys[i]) + 1;
  }

  // allocate buffer
  char* buf = malloc(length);

  // write names
  size_t pos = 0;
  for (int i = 0; i < NAOS_SETTING_MAX; i++) {
    // get param
    const char* name = naos_setting_keys[i];

    // copy name
    strcpy(buf + pos, name);
    pos += strlen(name);

    // write comma or zero
    buf[pos] = (char)((i == NAOS_SETTING_MAX - 1) ? '\0' : ',');
    pos++;
  }

  return buf;
}
