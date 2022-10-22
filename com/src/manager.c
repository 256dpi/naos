#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <string.h>

#ifndef CONFIG_NAOS_WIFI_DISABLE
#include <esp_wifi.h>
#endif

#include "coredump.h"
#include "manager.h"
#include "monitor.h"
#include "naos.h"
#include "params.h"
#include "settings.h"
#include "update.h"
#include "utils.h"

static SemaphoreHandle_t naos_manager_mutex;

static TaskHandle_t naos_manager_task;

static bool naos_manager_process_started = false;

static bool naos_manager_recording = false;

static void naos_manager_send_heartbeat() {
  // get device name
  char *device_name = naos_settings_read(NAOS_SETTING_DEVICE_NAME);

  // get battery level
  float battery_level = -1;
  if (naos_config()->battery_level != NULL) {
    battery_level = naos_config()->battery_level();
  }

  // get signal strength
  int8_t rssi = -1;
#ifndef CONFIG_NAOS_WIFI_DISABLE
  wifi_ap_record_t record = {0};
  esp_wifi_sta_get_ap_info(&record);
  rssi = record.rssi;
#endif

  // get CPU usage
  naos_cpu_usage_t usage = naos_monitor_get();

  // send heartbeat
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%d,%d,%s,%.2f,%d,%.2f,%.2f", naos_config()->device_type,
           naos_config()->firmware_version, device_name, esp_get_free_heap_size(), naos_millis(),
           esp_ota_get_running_partition()->label, battery_level, rssi, usage.cpu0, usage.cpu1);
  naos_publish("naos/heartbeat", buf, 0, false, NAOS_LOCAL);

  // free string
  free(device_name);
}

static void naos_manager_send_announcement() {
  // get device name & base topic
  char *device_name = naos_settings_read(NAOS_SETTING_DEVICE_NAME);
  char *base_topic = naos_settings_read(NAOS_SETTING_BASE_TOPIC);

  // send announce
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", naos_config()->device_type, naos_config()->firmware_version, device_name,
           base_topic);
  naos_publish("naos/announcement", buf, 0, false, NAOS_GLOBAL);

  // free strings
  free(device_name);
  free(base_topic);
}

static void naos_manager_process() {
  for (;;) {
    // acquire mutex
    NAOS_LOCK(naos_manager_mutex);

    // send heartbeat
    naos_manager_send_heartbeat();

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    // wait for next interval
    naos_delay(CONFIG_NAOS_HEARTBEAT_INTERVAL);
  }
}

void naos_manager_init() {
  // create mutex
  naos_manager_mutex = xSemaphoreCreateMutex();
}

void naos_manager_start() {
  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // check if already running
  if (naos_manager_process_started) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_manager_start: already started");
    NAOS_UNLOCK(naos_manager_mutex);
    return;
  }

  // set flag
  naos_manager_process_started = true;

  // create task
  ESP_LOGI(NAOS_LOG_TAG, "naos_manager_start: create task");
  xTaskCreatePinnedToCore(naos_manager_process, "naos-manager", 4096, NULL, 2, &naos_manager_task, 1);

  // subscribe to global topics
  naos_subscribe("naos/collect", 0, NAOS_GLOBAL);

  // subscribe to local topics
  naos_subscribe("naos/ping", 0, NAOS_LOCAL);
  naos_subscribe("naos/discover", 0, NAOS_LOCAL);
  naos_subscribe("naos/get/+", 0, NAOS_LOCAL);
  naos_subscribe("naos/set/+", 0, NAOS_LOCAL);
  naos_subscribe("naos/unset/+", 0, NAOS_LOCAL);
  naos_subscribe("naos/record", 0, NAOS_LOCAL);
  naos_subscribe("naos/debug", 0, NAOS_LOCAL);
  naos_subscribe("naos/update/begin", 0, NAOS_LOCAL);
  naos_subscribe("naos/update/write", 0, NAOS_LOCAL);
  naos_subscribe("naos/update/finish", 0, NAOS_LOCAL);

  // send initial announcement
  naos_manager_send_announcement();

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

void naos_manager_handle(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope) {
  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // check collect
  if (scope == NAOS_GLOBAL && strcmp(topic, "naos/collect") == 0) {
    // send announcement
    naos_manager_send_announcement();

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check ping
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/ping") == 0) {
    // call ping callback if present
    if (naos_config()->ping_callback != NULL) {
      naos_acquire();
      naos_config()->ping_callback();
      naos_release();
    }

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check discover
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/discover") == 0) {
    // get list
    char *list = naos_params_list();

    // send value
    naos_publish("naos/parameters", list, 0, false, NAOS_LOCAL);

    // free list
    free(list);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check get
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/get/", 9) == 0) {
    // get param
    char *param = (char *)topic + 9;

    // get value
    char *value = naos_get(param);

    // construct topic
    char *t = naos_str_concat("naos/value/", param);

    // send value
    naos_publish(t, value, 0, false, NAOS_LOCAL);

    // free topic
    free(t);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check set
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/set/", 9) == 0) {
    // get param
    char *param = (char *)topic + 9;

    // save param
    naos_set(param, (const char *)payload);

    // call update callback if present
    if (naos_config()->update_callback != NULL) {
      naos_acquire();
      naos_config()->update_callback(param, (const char *)payload);
      naos_release();
    }

    // construct topic
    char *t = naos_str_concat("naos/value/", param);

    // send value
    naos_publish(t, naos_get(param), 0, false, NAOS_LOCAL);

    // free topic
    free(t);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check unset
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/unset/", 11) == 0) {
    // get param
    char *param = (char *)topic + 11;

    // unset param and update task if it existed
    if (naos_unset(param)) {
      // call update callback if present
      if (naos_config()->update_callback != NULL) {
        naos_acquire();
        naos_config()->update_callback(param, NULL);
        naos_release();
      }
    }

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check log
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/record") == 0) {
    // enable or disable logging
    if (strcmp((const char *)payload, "on") == 0) {
      naos_manager_recording = true;
    } else if (strcmp((const char *)payload, "off") == 0) {
      naos_manager_recording = false;
    }

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check debug
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/debug") == 0) {
    // get coredump size
    uint32_t size = naos_coredump_size();
    if (size == 0) {
      naos_publish("naos/coredump", "", 0, false, NAOS_LOCAL);
      NAOS_UNLOCK(naos_manager_mutex);
      return;
    }

    // allocate buffer
    uint8_t *buf = malloc(CONFIG_NAOS_DEBUG_MAX_CHUNK_SIZE);

    // send coredump
    uint32_t sent = 0;
    while (sent < size) {
      // calculate next chunk size
      uint32_t chunk = CONFIG_NAOS_DEBUG_MAX_CHUNK_SIZE;
      if (size - sent < CONFIG_NAOS_DEBUG_MAX_CHUNK_SIZE) {
        chunk = size - sent;
      }

      // read chunk
      naos_coredump_read(sent, chunk, buf);

      // publish chunk
      naos_publish_r("naos/coredump", buf, chunk, 0, false, NAOS_LOCAL);

      // increment counter
      sent += chunk;
    }

    // free buffer
    free(buf);

    // clear if requested
    if (len == 6 && strcmp((const char *)payload, "delete") == 0) {
      naos_coredump_delete();
    }

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check update begin
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/begin") == 0) {
    // get update size
    size_t total = (size_t)strtol((const char *)payload, NULL, 10);
    ESP_LOGI(NAOS_LOG_TAG, "naos_manager_handle: begin update with size %zu", total);

    // begin update
    naos_update_begin(total);

    // request first chunk
    naos_publish("naos/update/request", naos_i2str(CONFIG_NAOS_UPDATE_MAX_CHUNK_SIZE), 0, false, NAOS_LOCAL);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check update write
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/write") == 0) {
    // write chunk (very time expensive)
    naos_update_write(payload, len);
    ESP_LOGI(NAOS_LOG_TAG, "naos_manager_handle: wrote chunk of %zu bytes", len);

    // request next chunk
    naos_publish("naos/update/request", naos_i2str(CONFIG_NAOS_UPDATE_MAX_CHUNK_SIZE), 0, false, NAOS_LOCAL);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check update finish
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/finish") == 0) {
    // finish update
    naos_update_finish();

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // call message callback if present
  if (naos_config()->message_callback != NULL) {
    naos_acquire();
    naos_config()->message_callback(topic, payload, len, scope);
    naos_release();
  }

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

char *naos_manager_read_param(naos_param_t *param) {
  // check param
  if (param == NULL) {
    return NULL;
  }

  // get param
  return strdup(naos_get(param->name));
}

void naos_manager_write_param(naos_param_t *param, const char *value) {
  // check number
  if (param == NULL) {
    return;
  }

  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // save param
  naos_set(param->name, value);

  // call update callback if present
  if (naos_config()->update_callback != NULL) {
    naos_acquire();
    naos_config()->update_callback(param->name, value);
    naos_release();
  }

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

void naos_log(const char *fmt, ...) {
  // prepare args
  va_list args;

  // initialize list
  va_start(args, fmt);

  // process input
  char buf[128];
  vsprintf(buf, fmt, args);

  // publish log message if enabled
  if (naos_manager_recording) {
    naos_publish("naos/log", buf, 0, false, NAOS_LOCAL);
  }

  // get device type
  const char *device_type = "unknown";
  if (naos_config() != NULL) {
    device_type = naos_config()->device_type;
  }

  // print log message esp like
  printf("N (%d) %s: %s\n", naos_millis(), device_type, buf);

  // free list
  va_end(args);
}

void naos_manager_stop() {
  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // check if task is still running
  if (!naos_manager_process_started) {
    NAOS_UNLOCK(naos_manager_mutex);
    return;
  }

  // set flags
  naos_manager_process_started = false;
  naos_manager_recording = false;

  // remove task
  ESP_LOGI(NAOS_LOG_TAG, "naos_manager_stop: deleting task");
  vTaskDelete(naos_manager_task);

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}
