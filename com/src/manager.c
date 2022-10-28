#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <string.h>

#include "naos.h"
#include "coredump.h"
#include "params.h"
#include "update.h"
#include "utils.h"
#include "com.h"

static SemaphoreHandle_t naos_manager_mutex;
static TaskHandle_t naos_manager_task;
static bool naos_manager_process_started = false;
static bool naos_manager_recording = false;

static void naos_manager_send_heartbeat() {
  // get device name
  char *device_name = strdup(naos_get("device-name"));

  // get battery
  float battery = -1;
  if (naos_config()->battery_callback != NULL) {
    battery = naos_config()->battery_callback();
  }

  // get WiFi RSSI
  int32_t rssi = -1;
  if (naos_lookup("wifi-rssi")) {
    rssi = naos_get_l("wifi-rssi");
  }

  // get CPU usage
  double cpu0 = 0, cpu1 = 0;
  if (naos_lookup("cpu-usage0")) {
    cpu0 = naos_get_d("cpu-usage0");
    cpu1 = naos_get_d("cpu-usage1");
  }

  // send heartbeat
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%d,%d,%s,%.2f,%d,%.2f,%.2f", naos_config()->device_type,
           naos_config()->device_version, device_name, esp_get_free_heap_size(), naos_millis(),
           esp_ota_get_running_partition()->label, battery, rssi, cpu0, cpu1);
  naos_publish("naos/heartbeat", buf, 0, false, NAOS_LOCAL);

  // free string
  free(device_name);
}

static void naos_manager_send_announcement() {
  // get device name & base topic
  char *device_name = strdup(naos_get("device-name"));
  char *base_topic = strdup(naos_get("mqtt-base-topic"));

  // send announce
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", naos_config()->device_type, naos_config()->device_version, device_name,
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

static void naos_manager_receiver(naos_param_t *param) {
  // skip system params
  if (param->mode & NAOS_SYSTEM) {
    return;
  }

  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // call update callback if present
  if (naos_config()->update_callback != NULL) {
    naos_acquire();
    naos_config()->update_callback(param->name, param->value);
    naos_release();
  }

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

static void naos_manager_handler(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                 bool retained) {
  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // call message callback if present for non system messages
  if (strncmp(topic, "naos/", 5) != 0 && naos_config()->message_callback != NULL) {
    naos_acquire();
    naos_config()->message_callback(topic, payload, len, scope);
    naos_release();
  }

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
    char *list = naos_params_list(NAOS_APPLICATION);

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
    const char *value = naos_get(param);

    // construct topic
    char *t = naos_concat("naos/value/", param);

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

    // send value
    char *t = naos_concat("naos/value/", param);
    naos_publish(t, naos_get(param), 0, false, NAOS_LOCAL);
    free(t);

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

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

void naos_manager_init() {
  // create mutex
  naos_manager_mutex = xSemaphoreCreateMutex();

  // subscribe parameters changes
  naos_params_subscribe(naos_manager_receiver);

  // subscribe messages
  naos_com_subscribe(naos_manager_handler);
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

void naos_log(const char *fmt, ...) {
  // process input
  va_list args;
  va_start(args, fmt);
  char buf[128];
  vsnprintf(buf, 128, fmt, args);
  va_end(args);

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
