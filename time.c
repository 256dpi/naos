#include <freertos/FreeRTOS.h>
#include <freertos/task.h>

uint32_t nadk_millis() { return xTaskGetTickCount() * portTICK_PERIOD_MS; }

void nadk_sleep(int millis) {
  // TODO: Panic if used outside of the loop function.

  vTaskDelay(millis / portTICK_PERIOD_MS);
}
