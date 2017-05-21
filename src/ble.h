#ifndef _NADK_BLE_H
#define _NADK_BLE_H

/**
 * The available BLE attributes.
 */
typedef enum {
  NADK_BLE_ID_WIFI_SSID,
  NADK_BLE_ID_WIFI_PASSWORD,
  NADK_BLE_ID_MQTT_HOST,
  NADK_BLE_ID_MQTT_PORT,
  NADK_BLE_ID_MQTT_CLIENT_ID,
  NADK_BLE_ID_MQTT_USERNAME,
  NADK_BLE_ID_MQTT_PASSWORD,
  NADK_BLE_ID_DEVICE_TYPE,
  NADK_BLE_ID_DEVICE_NAME,
  NADK_BLE_ID_BASE_TOPIC,
  NADK_BLE_ID_CONNECTION_STATUS,
  NADK_BLE_ID_COMMAND
} nadk_ble_id_t;

/**
 * The attribute callback.
 */
typedef void (*nadk_ble_attribute_callback_t)(nadk_ble_id_t);

/**
 * Initialize the BLE subsystem.
 *
 * Note: Should only be called once on boot.
 *
 * @param cb - The attribute callback.
 * @param device_type - The device type.
 */
void nadk_ble_init(nadk_ble_attribute_callback_t cb, const char *device_type);

/**
 * Get the the string value of the characteristic with the supplied id.
 *
 * The caller is responsible to free the string after it has been used.
 *
 * @param id - The attribute id.
 * @return The copied string.
 */
char *nadk_ble_get_string(nadk_ble_id_t id);

/**
 * Set the the string value of the characteristic with the supplied id.
 *
 * @param id - The attribute id.
 * @param str - The new string value.
 */
void nadk_ble_set_string(nadk_ble_id_t id, const char *str);

#endif  // _NADK_BLE_H
