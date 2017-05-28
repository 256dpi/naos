#ifndef NADK_H
#define NADK_H

#include <stdbool.h>
#include <stdint.h>

/**
 * The messages scopes.
 *
 * The 'local' scope denotes messages that are transferred under the configured base topic of the device while the
 * 'global' scope denotes messages that are transferred directly below the root.
 */
typedef enum { NADK_LOCAL, NADK_GLOBAL } nadk_scope_t;

/**
 * The system statuses.
 */
typedef enum { NADK_DISCONNECTED, NADK_CONNECTED, NADK_NETWORKED } nadk_status_t;

/**
 * The main configuration object.
 */
typedef struct {
  /**
   * The device type.
   */
  const char *device_type;

  /**
   * The firmware version.
   */
  const char *firmware_version;

  /**
   * The callback that is called once the device comes online.
   */
  void (*online_callback)();

  /**
   * The message callback is called with incoming messages.
   *
   * Note: The base topic has already been removed from the topic and should not start with a '/'.
   *
   * @param topic
   * @param payload
   * @param len
   * @param scope
   */
  void (*message_callback)(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope);

  /**
   * The loop callback is called in over and over as long as the device is online.
   */
  void (*loop_callback)();

  /**
   * The interval of the loop callback in milliseconds.
   */
  int loop_interval;

  /**
   * The offline callback is called once the device becomes offline.
   */
  void (*offline_callback)();

  /**
   * The callback is called once the device has changed its status.
   */
  void (*status_callback)(nadk_status_t status);

  /**
   * If set, the device will randomly (up to 5s) delay startup to overcome WiFi and MQTT congestion issues if many
   * devices restart at the same time.
   */
  bool delay_startup;
} nadk_config_t;

/**
 * Initialize the NADK.
 *
 * Note: Should only be called once on boot.
 *
 * @param config - The configuration object.
 */
void nadk_init(nadk_config_t *config);

/**
 * Subscribe to specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is local.
 *
 * @param topic
 * @param scope
 * @return
 */
bool nadk_subscribe(const char *topic, int qos, nadk_scope_t scope);

/**
 * Unsubscribe from specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is local.
 *
 * @param topic
 * @param scope
 * @return
 */
bool nadk_unsubscribe(const char *topic, nadk_scope_t scope);

/**
 * Publish bytes payload to specified topic.
 *
 * The topic is automatically prefixed with the configured base topic if the scope is local.
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
 * The topic is automatically prefixed with the configured base topic if the scope is local.
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
 * The topic is automatically prefixed with the configured base topic if the scope is local.
 *
 * @param topic
 * @param num
 * @param qos
 * @param retained
 * @param scope
 * @return
 */
bool nadk_publish_num(const char *topic, int num, int qos, bool retained, nadk_scope_t scope);

#endif  // NADK_H
