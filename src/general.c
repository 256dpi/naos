#include <freertos/FreeRTOS.h>
#include <freertos/task.h>

uint32_t nadk_millis() { return xTaskGetTickCount() * portTICK_PERIOD_MS; }

void nadk_delay(int millis) {
  if (millis >= portTICK_PERIOD_MS) {
    vTaskDelay(millis / portTICK_PERIOD_MS);
  } else {
    vTaskDelay(1);
  }
}
