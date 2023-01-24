#include <naos_sys.h>

#include <esp_log.h>
#include <esp_debug_helpers.h>

int64_t naos_millis() {
  // return timestamp
  return (int64_t)esp_log_timestamp();
}

int64_t naos_micros() {
  // return timestamp
  return esp_timer_get_time();
}

void naos_delay(uint32_t millis) {
  if (millis >= portTICK_PERIOD_MS) {
    vTaskDelay(millis / portTICK_PERIOD_MS);
  } else {
    vTaskDelay(1);
  }
}

static void naos_execute(void *arg) {
  // run task
  ((naos_func_t)arg)();

  // delete task
  vTaskDelete(NULL);
}

naos_task_t naos_run(const char *name, uint16_t stack, int core, naos_func_t func) {
  // check core
  if (core < 0) {
    core = tskNO_AFFINITY;
  }

  // create task
  TaskHandle_t handle = {0};
  xTaskCreatePinnedToCore(naos_execute, name, stack, func, 2, &handle, core);

  return handle;
}

void naos_kill(naos_task_t task) {
  // delete task
  vTaskDelete(task);
}

void naos_repeat(const char *name, uint32_t millis, naos_func_t func) {
  // create and start timer
  TimerHandle_t timer = xTimerCreate(name, pdMS_TO_TICKS(millis), pdTRUE, 0, func);
  while (xTimerStart(timer, portMAX_DELAY) != pdPASS) {
  }
}

void naos_defer(naos_func_t func) {
  // pend function call
  while (xTimerPendFunctionCall(func, NULL, 0, portMAX_DELAY) != pdPASS) {
  }
}

naos_mutex_t naos_mutex() {
  // create mutex
  return xSemaphoreCreateMutex();
}

void naos_lock(naos_mutex_t mutex) {
  // acquire mutex
  while (xSemaphoreTake(mutex, pdMS_TO_TICKS(10000)) != pdPASS) {
    ESP_LOGW("NAOS", "naos_lock: was blocked for 10s");
    TaskHandle_t handle = xSemaphoreGetMutexHolder(mutex);
    if (handle != NULL) {
      ESP_LOGW("NAOS", "naos_lock: ==> task %s has lock", pcTaskGetName(handle));
    }
    esp_backtrace_print(100);
  }
}

void naos_unlock(naos_mutex_t mutex) {
  // release mutex
  xSemaphoreGive(mutex);
}

naos_signal_t naos_signal() {
  // create event group
  return xEventGroupCreate();
}

void naos_trigger(naos_signal_t signal, uint16_t bits) {
  // check bits
  if (bits == 0) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // set bits
  xEventGroupSetBits(signal, bits);
}

void naos_await(naos_signal_t signal, uint16_t bits, bool clear) {
  // check bits
  if (bits == 0) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // await bits
  while (xEventGroupWaitBits(signal, bits, clear, pdTRUE, portMAX_DELAY) == 0) {
  }
}
