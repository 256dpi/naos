#ifndef _NAOS_GENERAL_H
#define _NAOS_GENERAL_H

#include <stdint.h>

/**
 * The general log tag.
 */
#define NAOS_LOG_TAG "naos"

/**
 * Acquire the specified mutex.
 *
 * @param mutex - The mutex to be locked.
 */
#define NAOS_LOCK(mutex) \
  do {                   \
  } while (xSemaphoreTake(mutex, portMAX_DELAY) != pdPASS)

/**
 * Release the specified mutex.
 *
 * @param mutex - The mutex to be released.
 */
#define NAOS_UNLOCK(mutex) xSemaphoreGive(mutex)

/**
 * Convert number to string.
 *
 * Note: The returned string is valid until the next invocation.
 *
 * @param num - The number.
 * @return The number as a string.
 */
const char *naos_i2str(int32_t num);

/**
 * Convert number to string.
 *
 * Note: The returned string is valid until the next invocation.
 *
 * @param num - The number.
 * @return The number as a string.
 */
const char *naos_d2str(double num);

/**
 * Will concatenate two strings and return a new one.
 *
 * Note: The caller is responsible to free the returned buffer.
 *
 * @param str1 - The first string.
 * @param str2 - The second string.
 * @return The concatenated string.
 */
char *naos_str_concat(const char *str1, const char *str2);

#endif  // _NAOS_GENERAL_H
