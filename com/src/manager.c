#include <naos/sys.h>
#include <naos/update.h>
#include <naos/cpu.h>
#include <naos/wifi.h>

#include <esp_log.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <string.h>

#include "coredump.h"
#include "params.h"
#include "utils.h"
#include "com.h"
#include "log.h"
#include "system.h"

static naos_mutex_t naos_manager_mutex;
static bool naos_manager_recording = false;

static void naos_manager_heartbeat() {
  // get device name
  const char *device_name = naos_get_s("device-name");

  // get battery level
  double battery = -1;
  if (naos_lookup("battery")) {
    battery = naos_get_d("battery");
  }

  // get WiFi RSSI
  int8_t rssi = 0;
  naos_wifi_info(&rssi);

  // get CPU usage
  float cpu0 = 0, cpu1 = 0;
  naos_cpu_get(&cpu0, &cpu1);

  // send heartbeat
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%ld,%lld,%s,%.2f,%d,%.2f,%.2f", naos_config()->device_type,
           naos_config()->device_version, device_name, esp_get_free_heap_size(), naos_millis(),
           esp_ota_get_running_partition()->label, battery, rssi, cpu0, cpu1);
  naos_publish_s("naos/heartbeat", buf, 0, false, NAOS_LOCAL);
}

static void naos_manager_announce() {
  // get device name and base topic
  const char *device_name = naos_get_s("device-name");
  const char *base_topic = naos_get_s("base-topic");

  // send announcement
  char buf[64];
  snprintf(buf, sizeof buf, "%s,%s,%s,%s", naos_config()->device_type, naos_config()->device_version, device_name,
           base_topic);
  naos_publish_s("naos/announcement", buf, 0, false, NAOS_GLOBAL);
}

static void naos_manager_update(naos_update_event_t event) {
  // skip non-ready events
  if (event != NAOS_UPDATE_READY) {
    return;
  }

  // request first chunk
  naos_publish_l("naos/update/request", CONFIG_NAOS_UPDATE_MAX_CHUNK_SIZE, 0, false, NAOS_LOCAL);
}

static void naos_manager_handler(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                 bool retained) {
  // skip other messages
  if (strncmp(topic, "naos/", 5) != 0) {
    return;
  }

  // acquire mutex
  naos_lock(naos_manager_mutex);

  // handle collect
  if (scope == NAOS_GLOBAL && strcmp(topic, "naos/collect") == 0) {
    // send announcement
    naos_manager_announce();

    // release mutex
    naos_unlock(naos_manager_mutex);

    return;
  }

  // handle ping
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/ping") == 0) {
    // release mutex (conflict with naos_set_s)
    naos_unlock(naos_manager_mutex);

    // trigger ping
    if (naos_lookup("ping") != NULL) {
      naos_set_s("ping", "");
    }

    return;
  }

  // handle discover
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/discover") == 0) {
    // send list
    char *list = naos_params_list(NAOS_APPLICATION);
    naos_publish_s("naos/parameters", list, 0, false, NAOS_LOCAL);
    free(list);

    // release mutex
    naos_unlock(naos_manager_mutex);

    return;
  }

  // handle get
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/get/", 9) == 0) {
    // release mutex
    naos_unlock(naos_manager_mutex);

    // get param
    char *param = (char *)topic + 9;

    // check param
    if (naos_lookup(param) == NULL) {
      return;
    }

    // get value
    naos_value_t value = naos_get(param);

    // send value
    char *t = naos_concat("naos/value/", param);
    naos_publish(t, value.buf, value.len, 0, false, NAOS_LOCAL);
    free(t);

    return;
  }

  // handle set
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/set/", 9) == 0) {
    // release mutex (conflict with naos_set)
    naos_unlock(naos_manager_mutex);

    // get param
    char *param = (char *)topic + 9;

    // check param
    if (naos_lookup(param) == NULL) {
      return;
    }

    // set value
    naos_set(param, (uint8_t *)payload, len);

    // get value
    naos_value_t value = naos_get(param);

    // send value
    char *t = naos_concat("naos/value/", param);
    naos_publish(t, value.buf, value.len, 0, false, NAOS_LOCAL);
    free(t);

    return;
  }

  // handle unset
  if (scope == NAOS_LOCAL && strncmp(topic, "naos/unset/", 11) == 0) {
    // release mutex (conflict with naos_clear)
    naos_unlock(naos_manager_mutex);

    // get param
    char *param = (char *)topic + 11;

    // check param
    if (naos_lookup(param) == NULL) {
      return;
    }

    // clear value
    naos_clear(param);

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
    naos_unlock(naos_manager_mutex);

    return;
  }

  // handle debug
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/debug") == 0) {
    // get coredump size
    uint32_t size = naos_coredump_size();
    if (size == 0) {
      naos_publish_s("naos/coredump", "", 0, false, NAOS_LOCAL);
      naos_unlock(naos_manager_mutex);
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
      naos_publish("naos/coredump", buf, chunk, 0, false, NAOS_LOCAL);

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
    naos_unlock(naos_manager_mutex);

    return;
  }

  // handle update begin
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/begin") == 0) {
    // get update size
    size_t total = (size_t)strtol((const char *)payload, NULL, 10);
    ESP_LOGI(NAOS_LOG_TAG, "naos_manager_handle: begin update with size %zu", total);

    // begin update
    naos_update_begin(total, naos_manager_update);

    // release mutex
    naos_unlock(naos_manager_mutex);

    return;
  }

  // handle update write
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/write") == 0) {
    // write chunk
    naos_update_write(payload, len);
    ESP_LOGI(NAOS_LOG_TAG, "naos_manager_handle: wrote chunk of %zu bytes", len);

    // request next chunk
    naos_publish_l("naos/update/request", CONFIG_NAOS_UPDATE_MAX_CHUNK_SIZE, 0, false, NAOS_LOCAL);

    // release mutex
    naos_unlock(naos_manager_mutex);

    return;
  }

  // handle update finish
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/update/finish") == 0) {
    // finish update
    naos_update_finish();

    // release mutex
    naos_unlock(naos_manager_mutex);

    return;
  }

  // release mutex
  naos_unlock(naos_manager_mutex);
}

static void naos_manager_sink(const char *msg) {
  // acquire mutex
  naos_lock(naos_manager_mutex);

  // publish message if networked and recording
  if (naos_com_networked(NULL) && naos_manager_recording) {
    naos_publish_s("naos/log", msg, 0, false, NAOS_LOCAL);
  }

  // release mutex
  naos_unlock(naos_manager_mutex);
}

static void naos_manager_signal() {
  // acquire mutex
  naos_lock(naos_manager_mutex);

  // send heartbeat if networked
  if (naos_com_networked(NULL)) {
    naos_manager_heartbeat();
  }

  // release mutex
  naos_unlock(naos_manager_mutex);
}

static void naos_manager_status(naos_status_t status) {
  // acquire mutex
  naos_lock(naos_manager_mutex);

  // handle status
  if (status == NAOS_NETWORKED) {
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
  } else {
    // clear recording
    naos_manager_recording = false;
  }

  // release mutex
  naos_unlock(naos_manager_mutex);
}

void naos_manager_init() {
  // create mutex
  naos_manager_mutex = naos_mutex();

  // handle status
  naos_system_subscribe(naos_manager_status);

  // handle messages
  naos_com_subscribe(naos_manager_handler);

  // register sink
  naos_log_register(naos_manager_sink);

  // start signal timer
  naos_repeat("naos-manager", CONFIG_NAOS_HEARTBEAT_INTERVAL, naos_manager_signal);
}
