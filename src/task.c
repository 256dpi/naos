#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <string.h>

#include <nadk.h>

#include "general.h"
#include "task.h"

static SemaphoreHandle_t nadk_task_mutex;

static TaskHandle_t nadk_task_ref;

static bool nadk_task_started = false;

static void nadk_task_process(void *p) {
  for (;;) {
    // acquire mutex
    NADK_LOCK(nadk_task_mutex);

    // call loop callback
    nadk_config()->loop_callback();

    // release mutex
    NADK_UNLOCK(nadk_task_mutex);

    // yield to other processes
    nadk_sleep(nadk_config()->loop_interval);
  }
}

void nadk_task_init() {
  // create mutex
  nadk_task_mutex = xSemaphoreCreateMutex();
}

void nadk_task_start() {
  // acquire mutex
  NADK_LOCK(nadk_task_mutex);

  // check if already running
  if (nadk_task_started) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_task_start: already started");
    NADK_UNLOCK(nadk_task_mutex);
    return;
  }

  // set flag
  nadk_task_started = true;

  // call setup callback if present
  if (nadk_config()->online_callback) {
    nadk_config()->online_callback();
  }

  // create task if loop is present
  if (nadk_config()->loop_callback != NULL) {
    ESP_LOGI(NADK_LOG_TAG, "nadk_task_start: create task");
    xTaskCreatePinnedToCore(nadk_task_process, "nadk-task", 8192, NULL, 2, &nadk_task_ref, 1);
  }

  // release mutex
  NADK_UNLOCK(nadk_task_mutex);
}

void nadk_task_stop() {
  // acquire mutex
  NADK_LOCK(nadk_task_mutex);

  // check if started
  if (!nadk_task_started) {
    NADK_UNLOCK(nadk_task_mutex);
    return;
  }

  // set flag
  nadk_task_started = false;

  // run terminate callback if present
  if (nadk_config()->offline_callback) {
    nadk_config()->offline_callback();
  }

  // remove task if loop is present
  if (nadk_config()->loop_callback != NULL) {
    ESP_LOGI(NADK_LOG_TAG, "nadk_task_stop: deleting task");
    vTaskDelete(nadk_task_ref);
  }

  // release mutex
  NADK_UNLOCK(nadk_task_mutex);
}

void nadk_task_forward(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // return immediately if no handle function exists
  if (nadk_config()->message_callback == NULL) {
    return;
  }

  // acquire mutex
  NADK_LOCK(nadk_task_mutex);

  // call handle callback
  nadk_config()->message_callback(topic, payload, len, scope);

  // release mutex
  NADK_UNLOCK(nadk_task_mutex);
}
