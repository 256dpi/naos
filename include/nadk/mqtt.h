#ifndef NADK_MQTT_H
#define NADK_MQTT_H

#include <stdbool.h>
#include <stdint.h>

typedef enum { NADK_SCOPE_DEVICE, NADK_SCOPE_GLOBAL } nadk_scope_t;

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

#endif  // NADK_MQTT_H
