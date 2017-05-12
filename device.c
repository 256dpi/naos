#include <esp_log.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <nadk.h>

#include "device.h"
#include "general.h"

static SemaphoreHandle_t nadk_device_mutex;

static nadk_device_t *nadk_device;

static TaskHandle_t nadk_device_task;

static bool nadk_device_process_started = false;

// TODO: Add offline device loop.
// The callbacks could be: offline, connected, online, disconnected.

static void nadk_device_process(void *p) {
  // call setup function
  if (nadk_device->setup_fn) {
    // acquire mutex
    NADK_LOCK(nadk_device_mutex);

    // call setup callback
    nadk_device->setup_fn();

    // release mutex
    NADK_UNLOCK(nadk_device_mutex);
  }

  for (;;) {
    // acquire mutex
    NADK_LOCK(nadk_device_mutex);

    // call loop function
    nadk_device->loop_fn();

    // release mutex
    NADK_UNLOCK(nadk_device_mutex);

    // yield to other processes
    vTaskDelay(1);
  }
}

void nadk_device_init(nadk_device_t *device) {
  // set device reference
  nadk_device = device;

  // create mutex
  nadk_device_mutex = xSemaphoreCreateMutex();
}

void nadk_device_start() {
  // acquire mutex
  NADK_LOCK(nadk_device_mutex);

  // check if already running
  if (nadk_device_process_started) {
    ESP_LOGE(NADK_LOG_TAG, "nadk_manager_start_device_process: already started");
    NADK_UNLOCK(nadk_device_mutex);
    return;
  }

  // set flag
  nadk_device_process_started = true;

  // create task
  ESP_LOGI(NADK_LOG_TAG, "nadk_device_start: create task");
  xTaskCreatePinnedToCore(nadk_device_process, "core-device", 8192, NULL, 2, &nadk_device_task, 1);

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

  // run terminate function
  if (nadk_device->terminate_fn) {
    nadk_device->terminate_fn();
  }

  // release mutex
  NADK_UNLOCK(nadk_device_mutex);
}

void nadk_device_forward(const char *topic, const char *payload, unsigned int len) {
  if (nadk_device->handle_fn) {
    // acquire mutex
    NADK_LOCK(nadk_device_mutex);

    // forward message
    nadk_device->handle_fn(topic, payload, len);

    // release mutex
    NADK_UNLOCK(nadk_device_mutex);
  }
}
