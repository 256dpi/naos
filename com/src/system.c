#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <string.h>

#include "system.h"
#include "naos.h"
#include "net.h"
#include "params.h"
#include "task.h"
#include "update.h"
#include "utils.h"
#include "com.h"
#include "log.h"

SemaphoreHandle_t naos_system_mutex;
static naos_status_t naos_system_status;

static void naos_system_ping() {
  // check existence
  if (naos_config()->ping_callback == NULL) {
    return;
  }

  // perform ping
  naos_acquire();
  naos_config()->ping_callback();
  naos_release();
}

static naos_param_t naos_system_params[] = {
    {.name = "device-type", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_PUBLIC | NAOS_LOCKED},
    {.name = "device-version", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "device-name", .type = NAOS_STRING, .mode = NAOS_SYSTEM | NAOS_PUBLIC},
    {.name = "device-reboot", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = esp_restart},
    {.name = "connection-status", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "running-partition", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "uptime", .type = NAOS_LONG, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "free-heap", .type = NAOS_LONG, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "battery-level", .type = NAOS_DOUBLE, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "ping", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_system_ping},
};

static void naos_system_set_status(naos_status_t status) {
  // get name
  const char *name = naos_status_str(status);

  // change state
  NAOS_LOCK(naos_system_mutex);
  naos_system_status = status;
  NAOS_UNLOCK(naos_system_mutex);

  // set status
  naos_set("connection-status", name);

  // call status callback if present
  if (naos_config()->status_callback != NULL) {
    naos_acquire();
    naos_config()->status_callback(status);
    naos_release();
  }

  // log new status
  ESP_LOGI(NAOS_LOG_TAG, "naos_system_set_status: %s", name);
}

static void naos_system_task() {
  // prepare generation
  uint32_t old_generation = 0;

  // prepare updated
  static uint32_t params_updated = 0;

  for (;;) {
    // wait some time
    naos_delay(100);

    // get old status
    naos_status_t old_status = naos_system_status;

    // determine new status
    uint32_t new_generation = 0;
    bool connected = naos_net_connected(NULL);
    bool networked = naos_com_networked(&new_generation);
    naos_status_t new_status = NAOS_DISCONNECTED;
    if (connected && networked) {
      new_status = NAOS_NETWORKED;
    } else if (connected) {
      new_status = NAOS_CONNECTED;
    }

    // handle status change
    if (naos_system_status != new_status) {
      // set status
      naos_system_set_status(new_status);

      // manage task
      if (new_status == NAOS_NETWORKED) {
        naos_task_start();
      } else if (old_status == NAOS_NETWORKED) {
        naos_task_stop();
      }
    } else if (networked && new_generation > old_generation) {
      // restart task
      naos_task_stop();
      naos_task_start();
    }

    // update generation
    old_generation = new_generation;

    // update parameters
    if (naos_millis() > params_updated + 1000) {
      naos_set_l("uptime", (int32_t)naos_millis());
      naos_set_l("free-heap", (int32_t)esp_get_free_heap_size());
      if (naos_config()->battery_callback != NULL) {
        naos_acquire();
        float level = naos_config()->battery_callback();
        naos_release();
        naos_set_d("battery-level", level);
      }
      params_updated = naos_millis();
    }

    // dispatch parameters
    naos_params_dispatch();
  }
}

static void naos_setup_task() {
  // run callback
  naos_acquire();
  naos_config()->setup_callback();
  naos_release();

  // delete itself
  vTaskDelete(NULL);
}

void naos_system_init() {
  // delay startup by max 5000ms if set
  if (naos_config()->delay_startup) {
    uint32_t delay = esp_random() / 858994;
    ESP_LOGI(NAOS_LOG_TAG, "naos_system_init: delay startup by %dms", delay);
    naos_delay(delay);
  }

  // create mutex
  naos_system_mutex = xSemaphoreCreateMutex();

  // init parameters
  naos_params_init();

  // register system parameters
  for (size_t i = 0; i < (sizeof(naos_system_params) / sizeof(naos_system_params[0])); i++) {
    naos_register(&naos_system_params[i]);
  }

  // initialize system parameters
  naos_set("device-type", naos_config()->device_type);
  naos_set("device-version", naos_config()->device_version);
  naos_set("running-partition", esp_ota_get_running_partition()->label);

  // initialize subsystems
  naos_log_init();
  naos_net_init();
  naos_com_init();

  // init task
  naos_task_init();

  // initialize OTA
  naos_update_init();

  // set initial state
  naos_system_set_status(NAOS_DISCONNECTED);

  // register application parameters
  for (int i = 0; i < naos_config()->num_parameters; i++) {
    naos_register(&naos_config()->parameters[i]);
  }

  // run system task
  xTaskCreatePinnedToCore(naos_system_task, "naos-system", 4096, NULL, 2, NULL, 1);

  // run setup task if provided
  if (naos_config()->setup_callback) {
    xTaskCreatePinnedToCore(naos_setup_task, "naos-setup", 8192, NULL, 2, NULL, 1);
  }
}

naos_status_t naos_status() {
  // get status
  NAOS_LOCK(naos_system_mutex);
  naos_status_t status = naos_system_status;
  NAOS_UNLOCK(naos_system_mutex);

  return status;
}
