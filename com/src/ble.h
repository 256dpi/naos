#ifndef _NAOS_BLE_H
#define _NAOS_BLE_H

/**
 * The maximum number of supported connections.
 */
#define NAOS_BLE_MAX_CONNECTIONS CONFIG_BT_ACL_CONNECTIONS

/**
 * A single BLE connection.
 */
typedef struct {
  uint16_t id;
  bool connected;
  bool locked;
} naos_ble_conn_t;

/**
 * The available BLE characteristics.
 */
typedef enum {
  NAOS_BLE_CHAR_WIFI_SSID,
  NAOS_BLE_CHAR_WIFI_PASSWORD,
  NAOS_BLE_CHAR_MQTT_HOST,
  NAOS_BLE_CHAR_MQTT_PORT,
  NAOS_BLE_CHAR_MQTT_CLIENT_ID,
  NAOS_BLE_CHAR_MQTT_USERNAME,
  NAOS_BLE_CHAR_MQTT_PASSWORD,
  NAOS_BLE_CHAR_DEVICE_TYPE,
  NAOS_BLE_CHAR_DEVICE_NAME,
  NAOS_BLE_CHAR_BASE_TOPIC,
  NAOS_BLE_CHAR_CONNECTION_STATUS,
  NAOS_BLE_CHAR_BATTERY_LEVEL,
  NAOS_BLE_CHAR_COMMAND,
  NAOS_BLE_CHAR_PARAMS_LIST,
  NAOS_BLE_CHAR_PARAMS_SELECT,
  NAOS_BLE_CHAR_PARAMS_VALUE
} naos_ble_char_t;

/**
 * The read callback.
 *
 * @note The returned pointer will automatically be freed.
 *
 * @param ch The characteristic.
 */
typedef char *(*naos_ble_read_callback_t)(naos_ble_conn_t *conn, naos_ble_char_t ch);

/**
 * The write callback.
 *
 * @param ch The characteristic.
 * @param value The value.
 */
typedef void (*naos_ble_write_callback_t)(naos_ble_conn_t *conn, naos_ble_char_t ch, const char *value);

/**
 * Initialize the BLE subsystem.
 *
 * @note Should only be called once on boot.
 *
 * @param rcb The read callback.
 * @param wcb The write callback.
 */
void naos_ble_init(naos_ble_read_callback_t rcb, naos_ble_write_callback_t wcb);

/**
 * Notify connected clients about changed values.
 *
 * @param ch The characteristic.
 * @param value The value.
 */
void naos_ble_notify(naos_ble_char_t ch, const char *value);

#endif  // _NAOS_BLE_H
