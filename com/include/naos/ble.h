#ifndef NAOS_BLE_H
#define NAOS_BLE_H

#include <stdbool.h>
#include <stdint.h>

typedef struct {
  /**
   * Whether to use the whitelist feature to remember connected devices and
   * allow them to reconnect while denying scan/connect requests from others.
   *
   * @see naos_ble_start_pairing()
   * @see naos_ble_stop_pairing()
   */
  bool pseudo_pairing;

  /**
   * Whether to skip bluetooth initialization.
   */
  bool skip_bt_init;
} naos_ble_config_t;

/**
 * Initialize the Bluetooth Low Energy configuration subsystem.
 */
void naos_ble_init(naos_ble_config_t cfg);

/**
 * Enable pairing in pseudo-pairing mode.
 */
void naos_ble_enable_pairing();

/**
 * Disable pairing in pseudo-pairing mode.
 */
void naos_ble_disable_pairing();

/**
 * Wait for a new connection (all modes).
 *
 * @return Whether a connection was established or not.
 */
bool naos_ble_await_connection(int32_t timeout_ms);

#endif  // NAOS_BLE_H
