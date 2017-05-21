#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <string.h>

#include <nadk.h>

#include "device.h"
#include "general.h"

// TODO: Rename subsystem.

static SemaphoreHandle_t nadk_device_mutex;

static nadk_device_t *nadk_device_ref;

static TaskHandle_t nadk_device_task;

static bool nadk_device_process_started = false;

// TODO: Add offline device loop.
// The callbacks could be: offline, connected, online, disconnected.

static void nadk_device_process(void *p) {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // call setup callback i present
  if (nadk_device_ref->setup) {
    nadk_device_ref->setup();
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);

  for (;;) {
    // acquire mutex
    NADK_LOCK(nadk_device_mutex);

    // call loop callback if present
    if (nadk_device_ref->loop) {
      nadk_device_ref->loop();
    }

    // release mutex
    NADK_UNLOCK(nadk_device_mutex);

    // yield to other processes
    nadk_yield();
  }
}

void nadk_device_init(nadk_device_t *device) {
  // set device reference
  nadk_device_ref = device;

  // create mutex
  nadk_device_mutex = xSemaphoreCreateMutex();
}

nadk_device_t *nadk_device() { return nadk_device_ref; }

void nadk_device_start() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if already running
  if (nadk_device_process_started) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_device_start: already started");
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_process_started = true;

  // create task
  ESP_LOGI(NADK_LOG_TAG, "nadk_device_start: create task");
  xTaskCreatePinnedToCore(nadk_device_process, "nadk-device", 8192, NULL, 2, &nadk_device_task, 1);

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_stop() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if task is still running
  if (!nadk_device_process_started) {
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_process_started = false;

  // remove task
  ESP_LOGI(NADK_LOG_TAG, "nadk_device_stop: deleting task");
  vTaskDelete(nadk_device_task);

  // run terminate callback if present
  if (nadk_device_ref->terminate) {
    nadk_device_ref->terminate();
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_forward(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // call handle callback if present
  if (nadk_device_ref->handle) {
    nadk_device_ref->handle(topic, payload, len, scope);
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}
