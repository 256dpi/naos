#ifndef NAOS_BLE_H
#define NAOS_BLE_H

#include <stdbool.h>

typedef struct {
  /**
   * Whether to skip bluetooth initialization.
   */
  bool skip_bt_init;

  /**
   * Whether to send indications on update characteristic.
   */
  bool send_updates;
} naos_ble_config_t;

/**
 * Initialize the Bluetooth Low Energy configuration subsystem.
 */
void naos_ble_init(naos_ble_config_t cfg);

#endif  // NAOS_BLE_H
