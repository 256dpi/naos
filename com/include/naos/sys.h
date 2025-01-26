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
 * Returns the elapsed milliseconds or microseconds since boot.
 * Both values are monotonic and will not overflow in a lifetime.
 *
 * @return The elapsed milliseconds or microseconds.
 */
int64_t naos_millis();
int64_t naos_micros();

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
 * @param core The CPU core (0: sys, 1: app, -1: no affinity).
 * @param func The function.
 * @return A handle.
 */
naos_task_t naos_run(const char *name, uint16_t stack, int core, naos_func_t func);

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
 * Defer a function call to the background task from an ISR.
 *
 * @param func The function.
 * @return Whether the function was deferred.
 */
bool naos_defer_isr(naos_func_t func);

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
 * Deletes the specified mutex.
 *
 * @param mutex The mutex.
 */
void naos_mutex_delete(naos_mutex_t mutex);

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
 * Sets or clears the specified signal bits.
 *
 * @param signal The signal.
 * @param bits The bits.
 * @param clear Whether to clear the bits.
 */
void naos_trigger(naos_signal_t signal, uint16_t bits, bool clear);
void naos_trigger_isr(naos_signal_t signal, uint16_t bits, bool clear);

/**
 * Awaits triggering of the specified signal bits.
 *
 * @param signal The signal.
 * @param bits  The bits.
 * @param clear Whether the bits should be cleared.
 */
void naos_await(naos_signal_t signal, uint16_t bits, bool clear);

/**
 * Deletes the specified signal.
 *
 * @param signal The signal.
 */
void naos_signal_delete(naos_signal_t signal);

/**
 * A queue handle.
 */
typedef QueueHandle_t naos_queue_t;

/**
 * Creates and returns a queue.
 *
 * @param length The queue length.
 * @param size The item length.
 * @return The queue.
 */
naos_queue_t naos_queue(uint16_t length, uint16_t size);

/**
 * Pushes an item into the specified queue.
 *
 * @param queue The queue.
 * @param item The item.
 * @param timeout_ms The timeout in milliseconds or -1 for none.
 * @return Whether the item was pushed.
 */
bool naos_push(naos_queue_t queue, void *item, int32_t timeout_ms);
bool naos_push_isr(naos_queue_t queue, void *item);

/**
 * Pops an item from the specified queue.
 *
 * @param queue The queue.
 * @param item The item.
 * @param timeout_ms The timeout in milliseconds or -1 for none.
 * @return Whether the item was popped.
 */
bool naos_pop(naos_queue_t queue, void *item, int32_t timeout_ms);

/**
 * Returns the current length of the specified queue.
 *
 * @param queue The queue.
 * @return The length.
 */
size_t naos_queue_length(naos_queue_t queue);

/**
 * Deletes the specified queue.
 *
 * @param queue The queue.
 */
void naos_queue_delete(naos_queue_t queue);

#endif  // NAOS_SYS_H
