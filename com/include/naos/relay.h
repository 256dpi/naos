#ifndef NAOS_RELAY_H
#define NAOS_RELAY_H

/**
 * NAOS MESSAGE RELAY
 * ==================
 *
 * The relay system provides a mechanism to relay messages to downstream devices
 * connected to a upstream host device. This allows standard access to devices
 * connected to an internal bus or network using the NAOS tools and libraries.
 *
 * On the host device, the relay system is installed as an endpoint that
 * receives messages from the messaging system and relays them to the downstream
 * device. On the downstream device, the relay system is registered as a channel
 * that receives messages from the host device.
 *
 * Once a sessions has been linked to a device, it cannot be used for anything
 * else. When done, the session should be ended to break the link. Also a session
 * cannot be linked to multiple devices, but multiple session can be linked to
 * the same device.
 */

#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

/**
 * The relay meta information.
 *
 * @param mtu The carrier MTU.
 */
typedef struct {
  uint16_t mtu;
} naos_relay_meta_t;

/**
 * The host relay configuration.
 *
 * @param scan The function to enumerate downstream devices.
 * @param send The function to send a message downstream.
 */
typedef struct {
  uint64_t (*scan)();
  bool (*send)(uint8_t num, uint8_t *data, size_t len, naos_relay_meta_t meta);
} naos_relay_host_t;

/**
 * The device relay configuration.
 *
 * @param mtu The relay MTU.
 * @param send The function to send a message upstream.
 */
typedef struct {
  uint16_t mtu;
  bool (*send)(uint8_t *data, size_t len);
} naos_relay_device_t;

/**
 * Initialize the upstream host relay endpoint.
 */
void naos_relay_host_init(naos_relay_host_t config);

/**
 * Initialize the downstream device relay channel.
 */
void naos_relay_device_init(naos_relay_device_t config);

/**
 * Process an upstream message on the host.
 *
 * @param num The device number.
 * @param data The message data.
 * @param len The message length.
 */
void naos_relay_host_process(uint8_t num, uint8_t *data, size_t len);

/**
 * Process a downstream message on the device.
 *
 * @param data The message data.
 * @param len The message length.
 */
void naos_relay_device_process(uint8_t *data, size_t len, naos_relay_meta_t meta);

#endif  // NAOS_RELAY_H
