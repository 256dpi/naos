#ifndef NAOS_BRIDGE_H
#define NAOS_BRIDGE_H

/**
 * Installs the bridge channel for all com transports (MQTT, OSC).
 *
 * Incoming messages are received on the local "naos/inbox" topic and outgoing
 * messages are sent to the local "naos/outbox" topic.
 */
void naos_bridge_install();

#endif  // NAOS_BRIDGE_H
