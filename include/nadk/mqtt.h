#ifndef NADK_MQTT_H
#define NADK_MQTT_H

#include <stdbool.h>
#include <stdint.h>

/**
 * Subscribe to specified topic.
 *
 * Note: The topic is automatically prefixed with the configured base topic.
 *
 * @param topic
 * @return
 */
bool nadk_subscribe(const char *topic, int qos);

/**
 * Unsubscribe from specified topic.
 *
 * Note: The topic is automatically prefixed with the configured base topic.
 *
 * @param topic
 * @return
 */
bool nadk_unsubscribe(const char *topic);

/**
 * Publish bytes payload to specified topic.
 *
 * Note: The topic is automatically prefixed with the configured base topic.
 *
 * @param topic
 * @param payload
 * @param len
 * @param qos
 * @param retained
 * @return
 */
bool nadk_publish(const char *topic, void *payload, uint16_t len, int qos, bool retained);

/**
 * Publish string to specified topic.
 *
 * Note: The topic is automatically prefixed with the configured base topic.
 *
 * @param topic
 * @param payload
 * @param qos
 * @param retained
 * @return
 */
bool nadk_publish_str(const char *topic, const char *payload, int qos, bool retained);

#endif  // NADK_MQTT_H
