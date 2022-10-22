#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>

#include "naos.h"
#include "task.h"
#include "utils.h"

static SemaphoreHandle_t naos_task_mutex;

static TaskHandle_t naos_task_ref;

static bool naos_task_started = false;

static void naos_task_process() {
  for (;;) {
    // acquire mutex
    NAOS_LOCK(naos_task_mutex);

    // call loop callback
    naos_config()->loop_callback();

    // release mutex
    NAOS_UNLOCK(naos_task_mutex);

    // yield to other processes
    naos_delay(naos_config()->loop_interval);
  }
}

void naos_acquire() {
  // acquire mutex
  NAOS_LOCK(naos_task_mutex);
}

void naos_release() {
  // release mutex
  NAOS_UNLOCK(naos_task_mutex);
}

void naos_task_init() {
  // create mutex
  naos_task_mutex = xSemaphoreCreateMutex();
}

void naos_task_start() {
  // acquire mutex
  NAOS_LOCK(naos_task_mutex);

  // check if already running
  if (naos_task_started) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_task_start: already started");
    NAOS_UNLOCK(naos_task_mutex);
    return;
  }

  // set flag
  naos_task_started = true;

  // call setup callback if present
  if (naos_config()->online_callback) {
    naos_config()->online_callback();
  }

  // create task if loop is present
  if (naos_config()->loop_callback != NULL) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_task_start: create task");
    xTaskCreatePinnedToCore(naos_task_process, "naos-task", 8192, NULL, 2, &naos_task_ref, 1);
  }

  // release mutex
  NAOS_UNLOCK(naos_task_mutex);
}

void naos_task_stop() {
  // acquire mutex
  NAOS_LOCK(naos_task_mutex);

  // check if started
  if (!naos_task_started) {
    NAOS_UNLOCK(naos_task_mutex);
    return;
  }

  // set flag
  naos_task_started = false;

  // run terminate callback if present
  if (naos_config()->offline_callback) {
    naos_config()->offline_callback();
  }

  // remove task if loop is present
  if (naos_config()->loop_callback != NULL) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_task_stop: deleting task");
    vTaskDelete(naos_task_ref);
  }

  // release mutex
  NAOS_UNLOCK(naos_task_mutex);
}
