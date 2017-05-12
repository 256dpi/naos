#ifndef _NADK_GENERAL_H
#define _NADK_GENERAL_H

/**
 * The log tag.
 */
#define NADK_LOG_TAG "CORE"

/**
 * Acquire the specified mutex.
 */
#define NADK_LOCK(x) \
  do {               \
  } while (xSemaphoreTake(x, portMAX_DELAY) != pdPASS)

/**
 * Release the specified mutex.
 */
#define NADK_UNLOCK(x) xSemaphoreGive(x)

#endif  // _NADK_GENERAL_H
