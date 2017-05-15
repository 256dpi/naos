#ifndef NADK_H
#define NADK_H

#include <stdbool.h>
#include <stdint.h>

typedef enum { NADK_SCOPE_LOCAL, NADK_SCOPE_GLOBAL } nadk_scope_t;

/**
 * Subscribe to specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is global.
 *
 * @param topic
 * @param scope
 * @return
 */
bool nadk_subscribe(const char *topic, int qos, nadk_scope_t scope);

/**
 * Unsubscribe from specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is global.
 *
 * @param topic
 * @param scope
 * @return
 */
bool nadk_unsubscribe(const char *topic, nadk_scope_t scope);

/**
 * Publish bytes payload to specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is global.
 *
 * @param topic
 * @param payload
 * @param len
 * @param qos
 * @param retained
 * @param scope
 * @return
 */
bool nadk_publish(const char *topic, void *payload, uint16_t len, int qos, bool retained, nadk_scope_t scope);

/**
 * Publish string to specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is global.
 *
 * @param topic
 * @param str
 * @param qos
 * @param retained
 * @param scope
 * @return
 */
bool nadk_publish_str(const char *topic, const char *str, int qos, bool retained, nadk_scope_t scope);

/**
 * Publish number to specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is global.
 *
 * @param topic
 * @param num
 * @param qos
 * @param retained
 * @param scope
 * @return
 */
bool nadk_publish_num(const char *topic, int num, int qos, bool retained, nadk_scope_t scope);

/**
 * Declaration of a NADK device.
 */
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

/**
 * Returns the elapsed milliseconds since the start.
 *
 * @return
 */
uint32_t nadk_millis();

/**
 * Will sleep for the specified amount of milliseconds.
 *
 * Note: This function should only be used inside the device loop function.
 *
 * @param millis
 */
void nadk_sleep(int millis);

#endif  // NADK_H
