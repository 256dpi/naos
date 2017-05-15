#ifndef NADK_H
#define NADK_H

#include <stdbool.h>
#include <stdint.h>

#include <nadk/mqtt.h>

typedef struct {
  /**
   * The device type.
   *
   * Will appear as the "device-type" characteristic over Bluetooth.
   */
  const char *type;

  /**
   * The callback that is called once the device comes online and should be used to subscribe to topics and do other
   * initialization work.
   */
  void (*setup_fn)();

  /**
   * The callback that should be called with incoming MQTT messages.
   *
   * Note: The base topic has already been removed from the topic and should not start with a '/'.
   *
   * @param topic
   * @param payload
   * @param len
   * @param scope
   */
  void (*handle_fn)(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope);

  /**
   * The callback that is called in a high frequency to do any necessary work of the device.
   */
  void (*loop_fn)();

  /**
   * The callback that is called once the device becomes offline.
   */
  void (*terminate_fn)();
} nadk_device_t;

/**
 * Initialize the NADK device management system.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device to be managed.
 */
void nadk_init(nadk_device_t *device);

#endif  // NADK_H
