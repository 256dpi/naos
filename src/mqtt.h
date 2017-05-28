#ifndef _NADK_MQTT_H
#define _NADK_MQTT_H

#include <esp_mqtt.h>

#include <nadk.h>

/**
 * The message callback.
 */
typedef void (*nadk_mqtt_message_callback_t)(const char *topic, const char *payload, unsigned int len,
                                             nadk_scope_t scope);

/**
 * Initialize the MQTT subsystem.
 *
 * Note: Should only be called once on boot.
 *
 * @param scb - The status callback.
 * @param mcb - The message callback.
 */
void nadk_mqtt_init(esp_mqtt_status_callback_t scb, nadk_mqtt_message_callback_t mcb);

/**
 * Start the MQTT process.
 *
 * @param host - The broker host.
 * @param port - The broker port.
 * @param client_id - The client id.
 * @param username - The client username.
 * @param password - The client password.
 * @param base_topic - The base topic.
 */
void nadk_mqtt_start(const char *host, unsigned int port, const char *client_id, const char *username,
                     const char *password, const char *base_topic);

/**
 * Stop the MQTT process.
 */
void nadk_mqtt_stop();

#endif  // _NADK_MQTT_H
