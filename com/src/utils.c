#include <esp_log.h>
#include <esp_err.h>
#include <stdio.h>
#include <string.h>

#include "naos.h"
#include "utils.h"

const char *naos_i2str(char buf[16], int32_t num) {
  snprintf(buf, 16, "%d", num);
  return buf;
}

const char *naos_d2str(char buf[16], double num) {
  snprintf(buf, 16, "%.4f", num);
  return buf;
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
