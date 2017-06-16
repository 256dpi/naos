#ifndef _NAOS_BLE_H
#define _NAOS_BLE_H

/**
 * The available BLE attributes.
 */
typedef enum {
  NAOS_BLE_ID_WIFI_SSID,
  NAOS_BLE_ID_WIFI_PASSWORD,
  NAOS_BLE_ID_MQTT_HOST,
  NAOS_BLE_ID_MQTT_PORT,
  NAOS_BLE_ID_MQTT_CLIENT_ID,
  NAOS_BLE_ID_MQTT_USERNAME,
  NAOS_BLE_ID_MQTT_PASSWORD,
  NAOS_BLE_ID_DEVICE_TYPE,
  NAOS_BLE_ID_DEVICE_NAME,
  NAOS_BLE_ID_BASE_TOPIC,
  NAOS_BLE_ID_CONNECTION_STATUS,
  NAOS_BLE_ID_COMMAND
} naos_ble_id_t;

/**
 * The attribute callback.
 */
typedef void (*naos_ble_attribute_callback_t)(naos_ble_id_t);

/**
 * Initialize the BLE subsystem.
 *
 * Note: Should only be called once on boot.
 *
 * @param cb - The attribute callback.
 * @param device_type - The device type.
 */
void naos_ble_init(naos_ble_attribute_callback_t cb, const char *device_type);

/**
 * Get the the string value of the characteristic with the supplied id.
 *
 * The caller is responsible to free the string after it has been used.
 *
 * @param id - The attribute id.
 * @return The copied string.
 */
char *naos_ble_get_string(naos_ble_id_t id);

/**
 * Set the the string value of the characteristic with the supplied id.
 *
 * @param id - The attribute id.
 * @param str - The new string value.
 */
void naos_ble_set_string(naos_ble_id_t id, const char *str);

#endif  // _NAOS_BLE_H
