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
  NAOS_SETTING_MAX,
  NAOS_SETTING_UNKNOWN = -1
} naos_setting_t;

/**
 * Get the common key for a setting.
 *
 * @param setting The setting.
 * @return The key string.
 */
const char* naos_setting2key(naos_setting_t setting);

/**
 * Get the setting for a common key.
 *
 * @param key The key.
 * @return The setting.
 */
naos_setting_t naos_key2setting(const char* key);

/**
 * Will initialize the settings subsystem.
 */
void naos_settings_init();

/**
 * Will read a setting form storage. The returned pointer must be freed after usage.
 *
 * @param setting The requested setting.
 * @return A pointer to the string value.
 */
char* naos_settings_read(naos_setting_t setting);

/**
 * Will write a setting to storage.
 *
 * @param setting The setting.
 * @param value The value.
 */
void naos_settings_write(naos_setting_t setting, const char* value);
