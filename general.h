#ifndef _NADK_GENERAL_H
#define _NADK_GENERAL_H

/**
 * The NADK log tag.
 */
#define NADK_LOG_TAG "NADK"

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
 * Will yield to other running processes.
 */
void nadk_yield();

#endif  // _NADK_GENERAL_H
