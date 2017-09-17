#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <string.h>
#include <sys/time.h>

uint64_t naos_micros() {
  struct timeval tv;
  gettimeofday(&tv, NULL);
  return 1000000 * (uint64_t)tv.tv_sec + (uint64_t)tv.tv_usec;
}

uint64_t naos_millis() { return naos_micros() / 1000; }

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
