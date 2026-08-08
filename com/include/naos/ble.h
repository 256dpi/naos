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
  bool pairing;

  /**
   * Whether to use the peerlist feature to establish a long-term secure
   * connection with a device, allowing it to reconnect without pairing.
   *
   * Note: If either side "forgets" a peer, the connection might fail. Remove
   * the obsolete device from the bonding list on the client, and clear the
   * bonding list on the device.
   */
  bool bonding;

  /**
   * Whether to skip bluetooth initialization.
   */
  bool skip_bt_init;

  /**
   * The advertising interval range in milliseconds. If both are set, they
   * replace the default aggressive range of 20/40 ms. Longer intervals
   * reduce power consumption at the expense of discovery latency. Values
   * are clamped to the specification range of 20 to 10240 ms.
   */
  uint16_t adv_int_min_ms;
  uint16_t adv_int_max_ms;

  /**
   * Whether to adaptively manage connection parameters. When enabled, each
   * connection switches between three profiles based on usage: a slow profile
   * (~1 s effective interval) when no messaging session is active, a medium
   * profile (30-45 ms) while a session is open, and a fast profile (7.5-15 ms)
   * while data is transferred in bulk or at a sustained high rate. Upgrades
   * are applied within a few seconds, downgrades after several seconds of
   * lower usage. When disabled, connections always use the fast profile.
   */
  bool adaptive;
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
 * Enable pairing in pairing mode.
 */
void naos_ble_enable_pairing();

/**
 * Disable pairing in pairing mode.
 */
void naos_ble_disable_pairing();

/**
 * Counts the number of entries in the allowlist (pairings).
 *
 * @return The number of entries in the allowlist.
 */
int naos_ble_allowlist_length();

/**
 * Removes all entries from the allowlist (pairings).
 */
void naos_ble_allowlist_clear();

/**
 * Counts the number of entries in the peerlist (bonds).
 *
 * @return The number of entries in the peerlist.
 */
int naos_ble_peerlist_length();

/**
 * Remove all entries from the peerlist (bonds).
 */
void naos_ble_peerlist_clear();

#endif  // NAOS_BLE_H
