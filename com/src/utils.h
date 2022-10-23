#ifndef NAOS_UTILS_H
#define NAOS_UTILS_H

#include <esp_log.h>
#include <stdint.h>

#define NAOS_LOG_TAG "naos"

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

#ifdef CONFIG_NAOS_DEBUG_LOCKS
#define NAOS_UNLOCK(mutex)                             \
  ESP_LOGI(NAOS_LOG_TAG, "NAOS_UNLOCK: %s", __func__); \
  xSemaphoreGive(mutex)
#else
#define NAOS_UNLOCK(mutex) xSemaphoreGive(mutex)
#endif

const char *naos_i2str(int32_t num);
const char *naos_d2str(double num);
char *naos_format(char *fmt, ...);
char *naos_concat(const char *str1, const char *str2);

#endif  // NAOS_UTILS_H
