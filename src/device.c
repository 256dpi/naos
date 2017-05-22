#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <string.h>

#include <nadk.h>

#include "device.h"
#include "general.h"

// TODO: Rename subsystem.

// TODO: Add offline device loop?

static SemaphoreHandle_t nadk_device_mutex;

static TaskHandle_t nadk_device_task;

static bool nadk_device_started = false;

static void nadk_device_process(void *p) {
  for (;;) {
    // acquire mutex
    NADK_LOCK(nadk_device_mutex);

    // call loop callback
    nadk_device()->loop();

    // release mutex
    NADK_UNLOCK(nadk_device_mutex);

    // TODO: Allow setting a custom delay duration.

    // yield to other processes
    nadk_yield();
  }
}

void nadk_device_init() {
  // create mutex
  nadk_device_mutex = xSemaphoreCreateMutex();
}

void nadk_device_start() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if already running
  if (nadk_device_started) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_device_start: already started");
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_started = true;

  // call setup callback if present
  if (nadk_device()->setup) {
    nadk_device()->setup();
  }

  // create task if loop is present
  if (nadk_device()->loop != NULL) {
    ESP_LOGI(NADK_LOG_TAG, "nadk_device_start: create task");
    xTaskCreatePinnedToCore(nadk_device_process, "nadk-device", 8192, NULL, 2, &nadk_device_task, 1);
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_stop() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if started
  if (!nadk_device_started) {
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_started = false;

  // run terminate callback if present
  if (nadk_device()->terminate) {
    nadk_device()->terminate();
  }

  // remove task if loop is present
  if (nadk_device()->loop != NULL) {
    ESP_LOGI(NADK_LOG_TAG, "nadk_device_stop: deleting task");
    vTaskDelete(nadk_device_task);
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_forward(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // return immediately if no handle function exists
  if (nadk_device()->handle == NULL) {
    return;
  }

  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // call handle callback
  nadk_device()->handle(topic, payload, len, scope);

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}
