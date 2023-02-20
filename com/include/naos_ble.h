#ifndef NAOS_BLE_H
#define NAOS_BLE_H

typedef struct {
  /**
   * Whether to skip bluetooth initialization.
   */
  bool skip_bt_init;
} naos_ble_config_t;

/**
 * Initialize the Bluetooth Low Energy configuration subsystem.
 */
void naos_ble_init(naos_ble_config_t cfg);

#endif  // NAOS_BLE_H
