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
 * The read callback. The read callback will free the passed pointer.
 *
 * Note: Do not call other BLE APIs!
 *
 * @param id - The characteristic.
 */
typedef char *(*naos_ble_read_callback_t)(naos_ble_id_t);

/**
 * The write callback.
 *
 * Note: Do not call other BLE APIs!
 *
 * @param id - The characteristic.
 * @param value - The value.
 */
typedef void (*naos_ble_write_callback_t)(naos_ble_id_t id, const char *value);

/**
 * Initialize the BLE subsystem.
 *
 * Note: Should only be called once on boot.
 *
 * @param rcb - The read callback.
 * @param wcb - The write callback.
 */
void naos_ble_init(naos_ble_read_callback_t rcb, naos_ble_write_callback_t wcb);

/**
 * Notify connected clients about changed values
 *
 * @param id - The characteristic.
 * @param value - The value.
 */
void naos_ble_notify(naos_ble_id_t id, const char *value);

#endif  // _NAOS_BLE_H
