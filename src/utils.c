#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <string.h>

uint32_t naos_millis() { return xTaskGetTickCount() * portTICK_PERIOD_MS; }

void naos_delay(uint32_t millis) {
  if (millis >= portTICK_PERIOD_MS) {
    vTaskDelay(millis / portTICK_PERIOD_MS);
  } else {
    vTaskDelay(1);
  }
}

void naos_sleep(uint32_t us) { ets_delay_us(us); }

char *naos_str_concat(const char *str1, const char *str2) {
  // allocate new buffer
  char *str = malloc(strlen(str1) + strlen(str2) + 1);

  // copy first string
  strcpy(str, str1);

  // copy second string
  strcat(str, str2);

  return str;
}
