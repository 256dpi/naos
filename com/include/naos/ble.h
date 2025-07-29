#ifndef NAOS_BLE_H
#define NAOS_BLE_H

#include <stdbool.h>
#include <stdint.h>

typedef struct {
  /**
   * Whether to use the allowlist feature to remember connected devices and
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
 * Wait for a new connection (all modes).
 *
 * @return Whether a connection was established or not.
 */
bool naos_ble_await(int32_t timeout_ms);

/**
 * Counts the number of active connections.
 *
 * @return The number of active connections.
 */
int naos_ble_connections();

/**
 * Enable pairing in pseudo-pairing mode.
 */
void naos_ble_enable_pairing();

/**
 * Disable pairing in pseudo-pairing mode.
 */
void naos_ble_disable_pairing();

/**
 * Counts the number of entries in the allowlist.
 *
 * @return The number of entries in the allowlist.
 */
int naos_ble_allowlist_length();

/**
 * Removes all entries from the allowlist.
 */
void naos_ble_allowlist_clear();

#endif  // NAOS_BLE_H
