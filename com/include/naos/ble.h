#ifndef NAOS_BLE_H
#define NAOS_BLE_H

#include <stdbool.h>
#include <stdint.h>

typedef struct {
  /**
   * Whether to use manual (on-demand) advertisement.
   */
  bool on_demand;

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
 * Start manual advertisement (on-demand mode).
 */
void naos_ble_start_advertisement();

/**
 * Stop manual advertisement (on-demand mode).
 */
void naos_ble_stop_advertisement();

/**
 * Wait for a new connection (all modes).
 *
 * @return Whether a connection was established or not.
 */
bool naos_ble_await_connection(int32_t timeout_ms);

#endif  // NAOS_BLE_H
