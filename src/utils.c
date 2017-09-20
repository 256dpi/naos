#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <string.h>
#include <esp_log.h>

uint32_t naos_millis() { return esp_log_timestamp(); }

void naos_delay(uint32_t millis) {
  if (millis >= portTICK_PERIOD_MS) {
    vTaskDelay(millis / portTICK_PERIOD_MS);
  } else {
    vTaskDelay(1);
  }
}

char *naos_str_concat(const char *str1, const char *str2) {
  // allocate new buffer
  char *str = malloc(strlen(str1) + strlen(str2) + 1);

  // copy first string
  strcpy(str, str1);

  // copy second string
  strcat(str, str2);

  return str;
}
