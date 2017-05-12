#ifndef _NADK_BLE_H
#define _NADK_BLE_H

/**
 * The available size for stored strings.
 */
#define NADK_BLE_STRING_SIZE 33

/**
 * The available BLE attributes.
 */
typedef enum nadk_ble_id_t {
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
 * The attribute change callback.
 */
typedef void (*nadk_ble_callback_t)(nadk_ble_id_t);

/**
 * The BLE module initializer.
 *
 * Note: Function is not thread-safe.
 */
void nadk_ble_init(nadk_ble_callback_t cb, const char *device_type);

/**
 * Get the the string value of the characteristic with the supplied id.
 */
void nadk_ble_get_string(nadk_ble_id_t id, char *destination);

/**
 * Set the the string value of the characteristic with the supplied id.
 */
void nadk_ble_set_string(nadk_ble_id_t id, char *str);

#endif  // _NADK_BLE_H
