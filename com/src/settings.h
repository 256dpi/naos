/**
 * The available settings.
 */
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
} naos_setting_t;

/**
 * Will initialize the settings subsystem.
 */
void naos_settings_init();

/**
 * Will read a setting form storage. The returned pointer must be freed after usage.
 *
 * @param setting - The requested setting.
 * @return A pointer to the string value.
 */
char* naos_settings_read(naos_setting_t setting);

/**
 * Will write a setting to storage.
 *
 * @param setting - The setting.
 * @param value - The value.
 */
void naos_settings_write(naos_setting_t setting, const char* value);
