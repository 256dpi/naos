#ifndef NAOS_UTILS_H
#define NAOS_UTILS_H

#include <stdint.h>

/**
 * Returns the elapsed milliseconds since the start.
 *
 * @return - The elapsed milliseconds.
 */
uint32_t naos_millis();

/**
 * Will sleep for the specified amount of milliseconds.
 *
 * Note: This function should only be used inside the loop callback.
 *
 * @param millis - The amount of milliseconds to delay.
 */
void naos_delay(uint32_t ms);

#endif  // NAOS_UTILS_H
