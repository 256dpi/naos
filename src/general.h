#ifndef _NADK_GENERAL_H
#define _NADK_GENERAL_H

/**
 * The general log tag.
 */
#define NADK_LOG_TAG "nadk"

/**
 * Acquire the specified mutex.
 *
 * @param mutex - The mutex to be locked.
 */
#define NADK_LOCK(mutex) \
  do {                   \
  } while (xSemaphoreTake(mutex, portMAX_DELAY) != pdPASS)

/**
 * Release the specified mutex.
 *
 * @param mutex - The mutex to be released.
 */
#define NADK_UNLOCK(mutex) xSemaphoreGive(mutex)

/**
 * Will sleep for the specified amount of milliseconds.
 *
 * @param millis
 */
void nadk_delay(int millis);

#endif  // _NADK_GENERAL_H
