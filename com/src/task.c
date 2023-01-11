#include <naos_sys.h>

#include <esp_log.h>

#include "naos.h"
#include "utils.h"
#include "system.h"
#include "params.h"
#include "com.h"

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

static void naos_task_status(naos_status_t status) {
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
      naos_task_handle = naos_run("naos-task", 8192, 1, naos_task_process);
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

static void naos_task_update(naos_param_t *param) {
  // skip system parameters
  if (param->mode & NAOS_SYSTEM) {
    return;
  }

  // yield update
  NAOS_LOCK(naos_task_mutex);
  naos_config()->update_callback(param->name, param->value);
  NAOS_UNLOCK(naos_task_mutex);
}

static void naos_task_message(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                              bool retained) {
  // yield message
  NAOS_LOCK(naos_task_mutex);
  naos_config()->message_callback(topic, payload, len, scope);
  NAOS_UNLOCK(naos_task_mutex);
}

static void naos_task_setup() {
  // run callback
  NAOS_LOCK(naos_task_mutex);
  naos_config()->setup_callback();
  NAOS_UNLOCK(naos_task_mutex);
}

static void naos_task_battery() {
  // update battery
  NAOS_LOCK(naos_task_mutex);
  float level = naos_config()->battery_callback();
  NAOS_UNLOCK(naos_task_mutex);
  naos_set_d("battery", level);
}

static void naos_task_ping() {
  // perform ping
  NAOS_LOCK(naos_task_mutex);
  naos_config()->ping_callback();
  NAOS_UNLOCK(naos_task_mutex);
}

static naos_param_t naos_task_param_battery = {
    .name = "battery",
    .type = NAOS_DOUBLE,
    .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED,
};

static naos_param_t naos_task_param_ping = {
    .name = "ping",
    .type = NAOS_ACTION,
    .mode = NAOS_SYSTEM,
    .func_a = naos_task_ping,
};

void naos_start() {
  // create mutex
  naos_task_mutex = naos_mutex();

  // register battery parameter if available
  if (naos_config()->battery_callback != NULL) {
    naos_register(&naos_task_param_battery);
    naos_repeat("battery", 1000, naos_task_battery);
  }

  // register ping parameter if available
  if (naos_config()->ping_callback != NULL) {
    naos_register(&naos_task_param_ping);
  }

  // subscribe status
  naos_system_subscribe(naos_task_status);

  // subscribe parameters if available
  if (naos_config()->update_callback != NULL) {
    naos_params_subscribe(naos_task_update);
  }

  // subscribe message if available
  if (naos_config()->message_callback != NULL) {
    naos_com_subscribe(naos_task_message);
  }

  // run setup task if provided
  if (naos_config()->setup_callback) {
    naos_run("naos-setup", 8192, 1, naos_task_setup);
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
