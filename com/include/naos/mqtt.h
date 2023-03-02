#ifndef NAOS_MQTT_H
#define NAOS_MQTT_H

/**
 * Initialize the MQTT communication transport.
 *
 * @param core The core to run the background task on.
 */
void naos_mqtt_init(int core);

#endif  // NAOS_MQTT_H
