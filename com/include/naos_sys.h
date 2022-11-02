#ifndef NAOS_SYS_H
#define NAOS_SYS_H

#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <freertos/semphr.h>
#include <freertos/event_groups.h>

/**
 * Locks a mutex with logs.
 */
#define NAOS_LOCK(mutex)                     \
  ESP_LOGV("NAOS", "LOCKING: %s", __func__); \
  naos_lock(mutex);                          \
  ESP_LOGV("NAOS", "LOCKED: %s", __func__)

/**
 * Unlocks a mutex with logs.
 */
#define NAOS_UNLOCK(mutex)                    \
  ESP_LOGV("NAOS", "UNLOCKED: %s", __func__); \
  naos_unlock(mutex)

/**
 * Returns the elapsed milliseconds since boot.
 *
 * @return The elapsed milliseconds.
 */
uint32_t naos_millis();

/**
 * Will delay the current task for the specified amount of milliseconds.
 *
 * @param ms The amount of milliseconds to delay.
 */
void naos_delay(uint32_t ms);

/**
 * A generic function.
 */
typedef void (*naos_func_t)();

/**
 * A task handle.
 */
typedef TaskHandle_t naos_task_t;

/**
 * Runs a task with the specified name and stack.
 *
 * @param name The name.
 * @param stack The stack size.
 * @param func The function.
 * @return A handle.
 */
naos_task_t naos_run(const char *name, uint16_t stack, naos_func_t func);

/**
 * Kill a task using the specified handle.
 *
 * @param task The handle.
 */
void naos_kill(naos_task_t task);

/**
 * Runs a periodic task using the specified name and period.
 *
 * @param name The name.
 * @param period_ms The period in milliseconds.
 * @param func The function.
 */
void naos_repeat(const char *name, uint32_t period_ms, naos_func_t func);

/**
 * Defer a function call to the background task.
 *
 * @param func The function.
 */
void naos_defer(naos_func_t func);

/**
 * A mutex handle.
 */
typedef SemaphoreHandle_t naos_mutex_t;

/**
 * Creates and returns a new mutex.
 */
naos_mutex_t naos_mutex();

/**
 * Locks the specified mutex.
 *
 * @param mutex The mutex.
 */
void naos_lock(naos_mutex_t mutex);

/**
 * Unlocks the specified mutex.
 *
 * @param mutex The mutex.
 */
void naos_unlock(naos_mutex_t mutex);

/**
 * A signal handle.
 */
typedef EventGroupHandle_t naos_signal_t;

/**
 * Creates and returns a signal.
 *
 * @return The signal.
 */
naos_signal_t naos_signal();

/**
 * Triggers the specified signal bits.
 *
 * @param signal The signal.
 * @param bits The bits.
 */
void naos_trigger(naos_signal_t signal, uint16_t bits);

/**
 * Awaits triggering of the specified signal bits.
 *
 * @param signal The signal.
 * @param bits  The bits.
 */
void naos_await(naos_signal_t signal, uint16_t bits);

#endif  // NAOS_SYS_H
