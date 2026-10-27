#ifndef NAOS_MQTT_H
#define NAOS_MQTT_H

#include <naos.h>
#include <stdbool.h>

/**
 * Initialize the MQTT communication transport.
 *
 * @param core The core to run the background task on.
 */
void naos_mqtt_init(int core);

/**
 * Set the last will that the broker publishes when the connection is lost
 * without a disconnect, e.g. due to a network outage or deep sleep. The will
 * applies from the next connection on, and a local topic is prefixed with the
 * base topic at that time. Must be called after naos_mqtt_init().
 *
 * @param topic The topic, or NULL to clear the will.
 * @param payload The payload.
 * @param qos The QoS level.
 * @param retained Whether the will is retained.
 * @param scope The topic scope.
 */
void naos_mqtt_lwt(const char *topic, const char *payload, int qos, bool retained, naos_scope_t scope);

#endif  // NAOS_MQTT_H
