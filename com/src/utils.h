#ifndef _NAOS_GENERAL_H
#define _NAOS_GENERAL_H

#include <esp_log.h>
#include <stdint.h>

/**
 * The general log tag.
 */
#define NAOS_LOG_TAG "naos"

/**
 * Acquire the specified mutex.
 *
 * @param mutex The mutex to be locked.
 */
#ifdef CONFIG_NAOS_DEBUG_LOCKS
#define NAOS_LOCK(mutex)                                    \
  ESP_LOGI(NAOS_LOG_TAG, "NAOS_LOCK: %s", __func__);        \
  do {                                                      \
  } while (xSemaphoreTake(mutex, portMAX_DELAY) != pdPASS); \
  ESP_LOGI(NAOS_LOG_TAG, "NAOS_LOCKED: %s", __func__)
#else
#define NAOS_LOCK(mutex) \
  do {                   \
  } while (xSemaphoreTake(mutex, portMAX_DELAY) != pdPASS)
#endif

/**
 * Release the specified mutex.
 *
 * @param mutex The mutex to be released.
 */
#ifdef CONFIG_NAOS_DEBUG_LOCKS
#define NAOS_UNLOCK(mutex)                             \
  ESP_LOGI(NAOS_LOG_TAG, "NAOS_UNLOCK: %s", __func__); \
  xSemaphoreGive(mutex)
#else
#define NAOS_UNLOCK(mutex) xSemaphoreGive(mutex)
#endif

/**
 * Convert number to string.
 *
 * @note The returned string is valid until the next invocation.
 *
 * @param num The number.
 * @return The number as a string.
 */
const char *naos_i2str(int32_t num);

/**
 * Convert number to string.
 *
 * @note The returned string is valid until the next invocation.
 *
 * @param num The number.
 * @return The number as a string.
 */
const char *naos_d2str(double num);

/**
 * Will concatenate two strings and return a new one.
 *
 * @note The caller is responsible to free the returned buffer.
 *
 * @param str1 The first string.
 * @param str2 The second string.
 * @return The concatenated string.
 */
char *naos_str_concat(const char *str1, const char *str2);

#endif  // _NAOS_GENERAL_H
