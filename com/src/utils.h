#ifndef NAOS_UTILS_H
#define NAOS_UTILS_H

#include <esp_log.h>
#include <stdint.h>

#define NAOS_LOG_TAG "naos"

#define NAOS_LOCK(mutex)                                    \
  ESP_LOGV(NAOS_LOG_TAG, "NAOS_LOCK: %s", __func__);        \
  do {                                                      \
  } while (xSemaphoreTake(mutex, portMAX_DELAY) != pdPASS); \
  ESP_LOGV(NAOS_LOG_TAG, "NAOS_LOCKED: %s", __func__)

#define NAOS_UNLOCK(mutex)                             \
  ESP_LOGV(NAOS_LOG_TAG, "NAOS_UNLOCK: %s", __func__); \
  xSemaphoreGive(mutex)

// naos_millis
// naos_delay

const char *naos_i2str(int32_t num);
const char *naos_d2str(double num);
char *naos_format(char *fmt, ...);
char *naos_concat(const char *str1, const char *str2);
void naos_repeat(const char *name, void (*task)(), uint32_t millis);

#endif  // NAOS_UTILS_H
