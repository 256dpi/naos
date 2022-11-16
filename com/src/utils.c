#include <esp_log.h>
#include <esp_err.h>
#include <stdio.h>
#include <string.h>
#include <float.h>

#include "utils.h"

const char *naos_i2str(char buf[16], int32_t num) {
  snprintf(str, 16, "%ld", num);
  return buf;
}

const char *naos_d2str(char buf[32], double num) {
  snprintf(buf, 32, "%.*g", DBL_DIG, num);
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

uint8_t *naos_copy(uint8_t *buf, size_t len) {
  // copy buffer and null terminate
  uint8_t *value = malloc(len + 1);
  memcpy(value, buf, len);
  value[len] = 0;

  return value;
}

char *naos_concat(const char *str1, const char *str2) {
  // copy strings
  char *str = malloc(strlen(str1) + strlen(str2) + 1);
  strcpy(str, str1);
  strcat(str, str2);

  return str;
}

bool naos_equal(uint8_t *buf, size_t len, const char *str) {
  // compare buffer to string
  if (strlen(str) == len) {
    return memcmp(buf, str, len) == 0;
  } else {
    return false;
  }
}
