#include <naos/sys.h>

#include <esp_log.h>
#include <esp_timer.h>
#include <esp_debug_helpers.h>
#include <esp_private/freertos_debug.h>

static void naos_backtrace_print(TaskHandle_t task, int depth) {
  // handle current task
  if (task == NULL) {
    esp_backtrace_print(depth);
    return;
  }

  // get task snapshot
  TaskSnapshot_t snapshot = {0};
  if (!vTaskGetSnapshot(task, &snapshot)) {
    return;
  }

  // get top of stack
  XtExcFrame *xtf = (XtExcFrame *)snapshot.pxTopOfStack;

  // prepare backtrace frame
  esp_backtrace_frame_t frame = {
      .pc = xtf->pc,
      .sp = xtf->a1,
      .next_pc = xtf->a0,
      .exc_frame = xtf,
  };

  // print backtrace
  esp_backtrace_print_from_frame(depth, &frame, false);
}

int64_t naos_millis() {
  // return timestamp
  return esp_timer_get_time() / 1000;
}

int64_t naos_micros() {
  // return timestamp
  return esp_timer_get_time();
}

void naos_delay(uint32_t millis) {
  // delay at least 1ms
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

void naos_repeat(const char *name, uint32_t period_ms, naos_func_t func) {
  // create and start timer
  TimerHandle_t timer = xTimerCreate(name, pdMS_TO_TICKS(period_ms), pdTRUE, 0, func);
  while (xTimerStart(timer, portMAX_DELAY) != pdPASS) {
  }
}

static void naos_defer_call(TimerHandle_t timer) {
  // call callback
  ((naos_func_t)pvTimerGetTimerID(timer))();

  // delete timer
  while (xTimerDelete(timer, portMAX_DELAY) != pdPASS) {
  }
}

void naos_defer(const char *name, uint32_t delay_ms, naos_func_t func) {
  // pend function call
  if (delay_ms == 0) {
    while (xTimerPendFunctionCall(func, NULL, 0, portMAX_DELAY) != pdPASS) {
    }
    return;
  }

  // create timer
  TimerHandle_t timer = xTimerCreate(name, pdMS_TO_TICKS(delay_ms), pdFALSE, func, naos_defer_call);
  if (timer == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // start timer
  xTimerStart(timer, portMAX_DELAY);
}

bool naos_defer_isr(naos_func_t func) {
  // pend function call
  return xTimerPendFunctionCallFromISR(func, NULL, 0, NULL) == pdPASS;
}

naos_mutex_t naos_mutex() {
  // create mutex
  return xSemaphoreCreateMutex();
}

void naos_lock(naos_mutex_t mutex) {
  // acquire mutex
  while (xSemaphoreTake(mutex, pdMS_TO_TICKS(10000)) != pdPASS) {
    // log error
    ESP_LOGE("NAOS", "naos_lock: was blocked for 10s");

    // get mutex holder
    TaskHandle_t holder = xSemaphoreGetMutexHolder(mutex);
    if (holder == NULL) {
      continue;
    }

    // print locker backtrace
    ESP_LOGE("NAOS", "======= LOCKER: %s =======", pcTaskGetName(NULL));
    naos_backtrace_print(NULL, 100);

    // print holder backtrace
    ESP_LOGE("NAOS", "======= HOLDER: %s =======", pcTaskGetName(holder));
    naos_backtrace_print(holder, 100);
  }
}

void naos_unlock(naos_mutex_t mutex) {
  // release mutex
  xSemaphoreGive(mutex);
}

void naos_mutex_delete(naos_mutex_t mutex) {
  // delete mutex
  vSemaphoreDelete(mutex);
}

naos_signal_t naos_signal() {
  // create event group
  return xEventGroupCreate();
}

void naos_trigger(naos_signal_t signal, uint16_t bits, bool clear) {
  // check bits
  if (bits == 0) {
    return;
  }

  // clear or set bits
  if (clear) {
    xEventGroupClearBits(signal, bits);
  } else {
    xEventGroupSetBits(signal, bits);
  }
}

void naos_trigger_isr(naos_signal_t signal, uint16_t bits, bool clear) {
  // check bits
  if (bits == 0) {
    return;
  }

  // clear or set bits
  if (clear) {
    xEventGroupClearBitsFromISR(signal, bits);
  } else {
    xEventGroupSetBitsFromISR(signal, bits, NULL);
  }
}

void naos_await(naos_signal_t signal, uint16_t bits, bool clear) {
  // check bits
  if (bits == 0) {
    return;
  }

  // await bits
  while (xEventGroupWaitBits(signal, bits, clear, pdTRUE, portMAX_DELAY) == 0) {
  }
}

void naos_signal_delete(naos_signal_t signal) {
  // delete event group
  vEventGroupDelete(signal);
}

naos_queue_t naos_queue(uint16_t length, uint16_t size) {
  // create queue
  return xQueueCreate(length, size);
}

bool naos_push(naos_queue_t queue, void *item, int32_t timeout_ms) {
  if (timeout_ms >= 0) {
    return xQueueSend(queue, item, timeout_ms / portTICK_PERIOD_MS) == pdPASS;
  }
  while (xQueueSend(queue, item, portMAX_DELAY) != pdPASS) {
  }
  return true;
}

bool naos_push_isr(naos_queue_t queue, void *item) { return xQueueSendFromISR(queue, item, NULL) == pdTRUE; }

bool naos_pop(naos_queue_t queue, void *item, int32_t timeout_ms) {
  if (timeout_ms >= 0) {
    return xQueueReceive(queue, item, timeout_ms / portTICK_PERIOD_MS) == pdPASS;
  }
  while (xQueueReceive(queue, item, portMAX_DELAY) != pdPASS) {
  }
  return true;
}

size_t naos_queue_length(naos_queue_t queue) {
  // return queue length
  return uxQueueMessagesWaiting(queue);
}

void naos_queue_delete(naos_queue_t queue) {
  // delete queue
  vQueueDelete(queue);
}
