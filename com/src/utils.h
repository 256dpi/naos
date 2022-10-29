#ifndef NAOS_UTILS_H
#define NAOS_UTILS_H

#include <esp_log.h>
#include <stdint.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <freertos/semphr.h>
#include <freertos/event_groups.h>

#define NAOS_LOG_TAG "naos"

#define NAOS_LOCK(mutex)                             \
  ESP_LOGV(NAOS_LOG_TAG, "NAOS_LOCK: %s", __func__); \
  naos_lock(mutex);                                  \
  ESP_LOGV(NAOS_LOG_TAG, "NAOS_LOCKED: %s", __func__)

#define NAOS_UNLOCK(mutex)                             \
  ESP_LOGV(NAOS_LOG_TAG, "NAOS_UNLOCK: %s", __func__); \
  naos_unlock(mutex)

// naos_millis
// naos_delay

const char *naos_i2str(int32_t num);
const char *naos_d2str(double num);
char *naos_format(char *fmt, ...);
char *naos_concat(const char *str1, const char *str2);

typedef TaskHandle_t naos_task_t;
typedef void (*naos_func_t)();
naos_task_t naos_run(const char *name, uint16_t stack, naos_func_t func);
void naos_kill(naos_task_t task);
void naos_repeat(const char *name, uint32_t millis, naos_func_t func);
void naos_defer(naos_func_t func);

typedef SemaphoreHandle_t naos_mutex_t;
naos_mutex_t naos_mutex();
void naos_lock(naos_mutex_t mutex);
void naos_unlock(naos_mutex_t mutex);

typedef EventGroupHandle_t naos_signal_t;
naos_signal_t naos_signal();
void naos_trigger(naos_signal_t signal, uint16_t bits);
void naos_await(naos_signal_t signal, uint16_t bits);

#endif  // NAOS_UTILS_H
