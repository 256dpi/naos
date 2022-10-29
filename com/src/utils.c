#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <freertos/timers.h>
#include <stdio.h>
#include <string.h>

#include "utils.h"

uint32_t naos_millis() {
  // return timestamp
  return esp_log_timestamp();
}

void naos_delay(uint32_t millis) {
  if (millis >= portTICK_PERIOD_MS) {
    vTaskDelay(millis / portTICK_PERIOD_MS);
  } else {
    vTaskDelay(1);
  }
}

const char *naos_i2str(int32_t num) {
  // convert
  static char str[16] = {0};
  snprintf(str, 16, "%d", num);

  return str;
}

const char *naos_d2str(double num) {
  // convert
  static char str[16] = {0};
  snprintf(str, 16, "%.4f", num);

  return str;
}

char *naos_format(char *fmt, ...) {
  // measure
  va_list va;
  va_start(va, fmt);
  int ret = vsnprintf(NULL, 0, fmt, va);
  va_end(va);

  // format
  char *buf = malloc(ret + 1);
  va_start(va, fmt);
  ret = vsnprintf(buf, ret + 1, fmt, va);
  va_end(va);
  if (ret < 0) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  return buf;
}

char *naos_concat(const char *str1, const char *str2) {
  // copy strings
  char *str = malloc(strlen(str1) + strlen(str2) + 1);
  strcpy(str, str1);
  strcat(str, str2);

  return str;
}

static void naos_execute(void *arg) {
  // run task
  ((naos_func_t)arg)();

  // delete task
  vTaskDelete(NULL);
}

void naos_run(const char *name, uint16_t stack, naos_func_t func) {
  // create task
  TaskHandle_t handle = {0};
  xTaskCreatePinnedToCore(naos_execute, name, stack, func, 2, &handle, 1);
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
