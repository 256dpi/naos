#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <stdio.h>
#include <string.h>

uint32_t naos_millis() { return esp_log_timestamp(); }

void naos_delay(uint32_t millis) {
  if (millis >= portTICK_PERIOD_MS) {
    vTaskDelay(millis / portTICK_PERIOD_MS);
  } else {
    vTaskDelay(1);
  }
}

const char *naos_i2str(int32_t num) {
  static char str[16] = {0};
  snprintf(str, 16, "%d", num);
  return str;
}

const char *naos_d2str(double num) {
  static char str[16] = {0};
  snprintf(str, 16, "%.4f", num);
  return str;
}

char *naos_format(char *fmt, ...) {
  va_list va;

  // get length
  va_start(va, fmt);
  int ret = vsnprintf(NULL, 0, fmt, va);
  va_end(va);

  // allocate
  char *buf = malloc(ret + 1);

  // format
  va_start(va, fmt);
  ret = vsnprintf(buf, ret + 1, fmt, va);
  va_end(va);

  // check ret
  if (ret < 0) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  return buf;
}

char *naos_concat(const char *str1, const char *str2) {
  // allocate new buffer
  char *str = malloc(strlen(str1) + strlen(str2) + 1);

  // copy first string
  strcpy(str, str1);

  // copy second string
  strcat(str, str2);

  return str;
}
