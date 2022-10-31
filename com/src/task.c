#include <naos_sys.h>

#include <esp_log.h>

#include "naos.h"
#include "utils.h"
#include "system.h"

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

static void naos_task_status(naos_status_t status, uint32_t generation) {
  // acquire lock
  NAOS_LOCK(naos_task_mutex);

  // stop task if started
  if (naos_task_started) {
    // run offline callback if available
    if (naos_config()->offline_callback) {
      naos_config()->offline_callback();
    }

    // kill task if loop is available
    if (naos_config()->loop_callback != NULL) {
      ESP_LOGI(NAOS_LOG_TAG, "naos_task_status: kill task");
      naos_kill(naos_task_handle);
    }

    // set flag
    naos_task_started = false;
  }

  // start task if newly networked or generation updated
  if (status == NAOS_NETWORKED) {
    // call online callback if available
    if (naos_config()->online_callback) {
      naos_config()->online_callback();
    }

    // create task if loop is available
    if (naos_config()->loop_callback != NULL) {
      ESP_LOGI(NAOS_LOG_TAG, "naos_task_start: run task");
      naos_task_handle = naos_run("naos-task", 8192, naos_task_process);
    }

    // set flag
    naos_task_started = true;
  }

  // call status callback if present
  if (naos_config()->status_callback != NULL) {
    naos_config()->status_callback(status);
  }

  // release lock
  NAOS_UNLOCK(naos_task_mutex);
}

static void naos_task_setup() {
  // run callback
  naos_acquire();
  naos_config()->setup_callback();
  naos_release();
}

void naos_start() {
  // create mutex
  naos_task_mutex = naos_mutex();

  // subscribe status
  naos_system_subscribe(naos_task_status);

  // run setup task if provided
  if (naos_config()->setup_callback) {
    naos_run("naos-setup", 8192, naos_task_setup);
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
