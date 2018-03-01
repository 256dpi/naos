#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/task.h>
#include <nvs.h>
#include <string.h>

#include "ble.h"
#include "coredump.h"
#include "manager.h"
#include "naos.h"
#include "task.h"
#include "update.h"
#include "utils.h"

static SemaphoreHandle_t naos_manager_mutex;

static nvs_handle naos_manager_nvs_handle;

static TaskHandle_t naos_manager_task;

static bool naos_manager_process_started = false;

static bool naos_manager_recording = false;

static const char *naos_manager_i2str(int num) {
  static char str[33];
  itoa(num, str, 10);
  return str;
}

static void naos_manager_send_heartbeat() {
  // get device name
  char *device_name = naos_ble_get_string(NAOS_BLE_ID_DEVICE_NAME);

  // send heartbeat
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%d,%d,%s", naos_config()->device_type, naos_config()->firmware_version,
           device_name, esp_get_free_heap_size(), naos_millis(), esp_ota_get_running_partition()->label);
  naos_publish("naos/heartbeat", buf, 0, false, NAOS_LOCAL);

  // free string
  free(device_name);
}

static void naos_manager_send_announcement() {
  // get device name & base topic
  char *device_name = naos_ble_get_string(NAOS_BLE_ID_DEVICE_NAME);
  char *base_topic = naos_ble_get_string(NAOS_BLE_ID_BASE_TOPIC);

  // send announce
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", naos_config()->device_type, naos_config()->firmware_version, device_name,
           base_topic);
  naos_publish("naos/announcement", buf, 0, false, NAOS_GLOBAL);

  // free strings
  free(device_name);
  free(base_topic);
}

static void naos_manager_process(void *p) {
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

  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-manager", NVS_READWRITE, &naos_manager_nvs_handle));
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
  xTaskCreatePinnedToCore(naos_manager_process, "naos-manager", 2048, NULL, 2, &naos_manager_task, 1);

  // TODO: Optimize by issuing all subscriptions at once.

  // subscribe to global topics
  naos_subscribe("naos/collect", 0, NAOS_GLOBAL);

  // subscribe to local topics
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

    // update task
    if (len > 0) {
      naos_task_update(param, (const char *)payload);
    } else {
      naos_task_update(param, "");
    }

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

  // check unset
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/unset/", 11) == 0) {
    // get param
    char *param = (char *)topic + 11;

    // unset param and update task if it existed
    if (naos_unset(param)) {
      naos_task_update(param, NULL);
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
      naos_publish_raw("naos/coredump", buf, chunk, 0, false, NAOS_LOCAL);

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
    long long int total = strtoll((const char *)payload, NULL, 10);
    ESP_LOGI(NAOS_LOG_TAG, "naos_manager_handle: begin update with size %lld", total);

    // begin update
    naos_update_begin((uint16_t)total);

    // request first chunk
    naos_publish("naos/update/request", naos_manager_i2str(CONFIG_NAOS_UPDATE_MAX_CHUNK_SIZE), 0, false, NAOS_LOCAL);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // check update write
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/write") == 0) {
    // write chunk (very time expensive)
    naos_update_write(payload, (uint16_t)len);
    ESP_LOGI(NAOS_LOG_TAG, "naos_manager_handle: wrote chunk of %zu bytes", len);

    // request next chunk
    naos_publish("naos/update/request", naos_manager_i2str(CONFIG_NAOS_UPDATE_MAX_CHUNK_SIZE), 0, false, NAOS_LOCAL);

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

  // if not handled, forward message to the task
  naos_task_forward(topic, payload, len, scope);

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

  // print log message esp like
  printf("N (%d) %s: %s\n", naos_millis(), naos_config()->device_type, buf);

  // free list
  va_end(args);
}

char *naos_get(const char *param) {
  // static reference to buffer
  static char *buf;

  // free last param
  if (buf != NULL) {
    free(buf);
    buf = NULL;
  }

  // get param size
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_manager_nvs_handle, param, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    buf = strdup("");
    return buf;
  } else {
    ESP_ERROR_CHECK(err);
  }

  // allocate size
  buf = malloc(required_size);
  ESP_ERROR_CHECK(nvs_get_str(naos_manager_nvs_handle, param, buf, &required_size));

  return buf;
}

void naos_set(const char *param, const char *value) {
  // set parameter
  ESP_ERROR_CHECK(nvs_set_str(naos_manager_nvs_handle, param, value));
}

bool naos_ensure(const char *param, const char *value) {
  // check parameter
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_manager_nvs_handle, param, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    naos_set(param, value);
    return true;
  } else {
    ESP_ERROR_CHECK(err);
  }

  return false;
}

bool naos_unset(const char *param) {
  // erase parameter
  esp_err_t err = nvs_erase_key(naos_manager_nvs_handle, param);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    return false;
  } else {
    ESP_ERROR_CHECK(err);
  }

  return true;
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
