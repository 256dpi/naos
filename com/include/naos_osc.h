#ifndef NAOS_OSC_H
#define NAOS_OSC_H

#include <esp_osc.h>

/**
 * Initialize the OSC communication transport.
 */
void naos_osc_init();

/**
 * Install a filter callback to pre-process messages before dispatch.
 *
 * @note: If false is returned from the callback, the message is not processed further.
 * */
void naos_osc_filter(esp_osc_callback_t filter);

/**
 * Send an OSC message to all configured targets.
 *
 * @param topic The topic.
 * @param format The format.
 * @param ... The values.
 * @return Whether the send was successful.
 */
bool naos_osc_send(const char *topic, const char *format, ...);

#endif  // NAOS_OSC_H
