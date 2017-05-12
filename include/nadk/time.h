#ifndef NADK_TIME_H
#define NADK_TIME_H

#include <stdint.h>

/**
 * Returns the elapsed milliseconds since the start.
 *
 * @return
 */
uint32_t nadk_millis();

/**
 * Will sleep for the specified amount of milliseconds.
 *
 * Note: This function should only be used inside the device loop function.
 *
 * @param millis
 */
void nadk_sleep(int millis);

#endif  // NADK_TIME_H
