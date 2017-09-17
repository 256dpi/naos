#ifndef NAOS_UTILS_H
#define NAOS_UTILS_H

#include <stdint.h>

/**
 * Returns the elapsed microseconds since the start.
 *
 * @return - The elapsed microseconds.
 */
uint64_t naos_micros();

/**
 * Returns the elapsed milliseconds since the start.
 *
 * @return - The elapsed milliseconds.
 */
uint64_t naos_millis();

/**
 * Will delay current task for the specified amount of milliseconds.
 *
 * Note: This function should only be used inside the loop callback.
 *
 * @param ms - The amount of milliseconds to delay.
 */
void naos_delay(uint32_t ms);

/**
 * Will sleep for the specified amount of milliseconds.
 *
 * Note: This function should be used carefully as it blocks all other processing.
 *
 * @param us - The amount of microseconds to sleep.
 */
void naos_sleep(uint32_t us);

#endif  // NAOS_UTILS_H
