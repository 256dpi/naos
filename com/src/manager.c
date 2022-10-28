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
#include "log.h"

static SemaphoreHandle_t naos_manager_mutex;
static bool naos_manager_recording = false;

static void naos_manager_heartbeat() {
  // get device name
  const char *device_name = naos_get("device-name");

  // get battery level
  double battery = -1;
  if (naos_lookup("battery-level")) {
    battery = naos_get_d("battery-level");
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
}

static void naos_manager_announce() {
  // get device name and base topic
  const char *device_name = naos_get("device-name");
  const char *base_topic = naos_get("base-topic");

  // send announcement
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", naos_config()->device_type, naos_config()->device_version, device_name,
           base_topic);
  naos_publish("naos/announcement", buf, 0, false, NAOS_GLOBAL);
}

static void naos_manager_handler(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                 bool retained) {
  // skip other messages
  if (strncmp(topic, "naos/", 5) != 0) {
    return;
  }

  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // handle collect
  if (scope == NAOS_GLOBAL && strcmp(topic, "naos/collect") == 0) {
    // send announcement
    naos_manager_announce();

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // handle ping
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/ping") == 0) {
    // release mutex (conflict with naos_set)
    NAOS_UNLOCK(naos_manager_mutex);

    // trigger ping
    naos_set("ping", "");

    return;
  }

  // handle discover
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/discover") == 0) {
    // send list
    char *list = naos_params_list(NAOS_APPLICATION);
    naos_publish("naos/parameters", list, 0, false, NAOS_LOCAL);
    free(list);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // handle get
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/get/", 9) == 0) {
    // get param
    char *param = (char *)topic + 9;

    // send value
    char *t = naos_concat("naos/value/", param);
    naos_publish(t, naos_get(param), 0, false, NAOS_LOCAL);
    free(t);

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // handle set
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/set/", 9) == 0) {
    // release mutex (conflict with naos_set)
    NAOS_UNLOCK(naos_manager_mutex);

    // get param
    char *param = (char *)topic + 9;

    // save param
    naos_set(param, (const char *)payload);

    // send value
    char *t = naos_concat("naos/value/", param);
    naos_publish(t, naos_get(param), 0, false, NAOS_LOCAL);
    free(t);

    return;
  }

  // handle record
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/record") == 0) {
    // enable or disable recording
    if (strcmp((const char *)payload, "on") == 0) {
      naos_manager_recording = true;
    } else if (strcmp((const char *)payload, "off") == 0) {
      naos_manager_recording = false;
    }

    // release mutex
    NAOS_UNLOCK(naos_manager_mutex);

    return;
  }

  // handle debug
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

  // handle update begin
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

  // handle update write
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

  // handle update finish
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

static void naos_manager_sink(const char *msg) {
  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // publish message if networked and recording
  if (naos_com_networked(NULL) && naos_manager_recording) {
    naos_publish("naos/log", msg, 0, false, NAOS_LOCAL);
  }

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

static void naos_manager_signal() {
  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // send heartbeat if networked
  if (naos_com_networked(NULL)) {
    naos_manager_heartbeat();
  }

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

static void naos_manager_check() {
  // keep old status
  static bool old_networked = false;
  static uint32_t old_generation = 0;

  // acquire mutex
  NAOS_LOCK(naos_manager_mutex);

  // get new status
  uint32_t new_generation = 0;
  bool new_networked = naos_com_networked(&new_generation);

  // handle status
  if ((!old_networked && new_networked) || (new_networked && new_generation > old_generation)) {
    // subscribe global topics
    naos_subscribe("naos/collect", 0, NAOS_GLOBAL);

    // subscribe local topics
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
    naos_manager_announce();
  } else if (!new_networked) {
    // clear recording
    naos_manager_recording = false;
  }

  // update status
  old_networked = new_networked;
  old_generation = new_generation;

  // release mutex
  NAOS_UNLOCK(naos_manager_mutex);
}

void naos_manager_init() {
  // create mutex
  naos_manager_mutex = xSemaphoreCreateMutex();

  // subscribe messages
  naos_com_subscribe(naos_manager_handler);

  // register sink
  naos_log_register(naos_manager_sink);

  // start signal timer
  naos_repeat("naos-manager#s", naos_manager_signal, CONFIG_NAOS_HEARTBEAT_INTERVAL);

  // start check timer
  naos_repeat("naos-manager#c", naos_manager_check, 100);
}
