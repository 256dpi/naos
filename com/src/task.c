#include <naos_sys.h>

#include <esp_log.h>

#include "naos.h"
#include "task.h"
#include "utils.h"

static naos_mutex_t naos_task_mutex;
static naos_task_t naos_task_handle;
static bool naos_task_started = false;

static void naos_task_process() {
  for (;;) {
    // call loop callback
    NAOS_LOCK(naos_task_mutex);
    naos_config()->loop_callback();
    NAOS_UNLOCK(naos_task_mutex);

    // yield to other processes
    naos_delay(naos_config()->loop_interval);
  }
}

void naos_task_init() {
  // create mutex
  naos_task_mutex = naos_mutex();
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

  // call online callback if present
  if (naos_config()->online_callback) {
    naos_config()->online_callback();
  }

  // create task if loop is present
  if (naos_config()->loop_callback != NULL) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_task_start: create task");
    naos_task_handle = naos_run("naos-task", 8192, naos_task_process);
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

  // run offline callback if present
  if (naos_config()->offline_callback) {
    naos_config()->offline_callback();
  }

  // remove task if loop is present
  if (naos_config()->loop_callback != NULL) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_task_stop: deleting task");
    naos_kill(naos_task_handle);
  }

  // release mutex
  NAOS_UNLOCK(naos_task_mutex);
}

void naos_acquire() {
  // acquire mutex
  NAOS_LOCK(naos_task_mutex);
}

void naos_release() {
  // release mutex
  NAOS_UNLOCK(naos_task_mutex);
}
