#ifndef NAOS_MQTT_H
#define NAOS_MQTT_H

#include <naos.h>

typedef struct {
  bool running;
  bool connected;
} naos_mqtt_status_t;

typedef void (*naos_mqtt_message_callback_t)(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope);

void naos_mqtt_init(naos_mqtt_message_callback_t mcb);
void naos_mqtt_start(const char *host, char *port, const char *client_id, const char *username, const char *password,
                     const char *base_topic);
naos_mqtt_status_t naos_mqtt_check();
void naos_mqtt_stop();

#endif  // NAOS_MQTT_H
