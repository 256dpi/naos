#ifndef NAOS_SETTINGS_H
#define NAOS_SETTINGS_H

typedef enum {
  NAOS_SETTING_WIFI_SSID,
  NAOS_SETTING_WIFI_PASSWORD,
  NAOS_SETTING_MQTT_HOST,
  NAOS_SETTING_MQTT_PORT,
  NAOS_SETTING_MQTT_CLIENT_ID,
  NAOS_SETTING_MQTT_USERNAME,
  NAOS_SETTING_MQTT_PASSWORD,
  NAOS_SETTING_DEVICE_NAME,
  NAOS_SETTING_BASE_TOPIC,
  NAOS_SETTING_MAX,
  NAOS_SETTING_UNKNOWN = -1
} naos_setting_t;

const char* naos_setting_to_key(naos_setting_t setting);
naos_setting_t naos_setting_from_key(const char* key);

void naos_settings_init();
char* naos_settings_read(naos_setting_t setting);
void naos_settings_write(naos_setting_t setting, const char* value);
char* naos_settings_list();

#endif  // NAOS_SETTINGS_H
