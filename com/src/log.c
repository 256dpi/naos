#include <naos.h>
#include <naos/sys.h>

#include <esp_log.h>
#include <esp_err.h>

#include "log.h"

#define NAOS_LOG_MAX_SINKS 8

static naos_mutex_t naos_log_mutex;
static naos_log_sink_t naos_log_sinks[NAOS_LOG_MAX_SINKS] = {0};
static size_t naos_log_sink_count = 0;

void naos_log_init() {
  // create mutex
  naos_log_mutex = naos_mutex();
}

void naos_log_register(naos_log_sink_t sink) {
  // acquire mutex
  naos_lock(naos_log_mutex);

  // check count
  if (naos_log_sink_count >= NAOS_LOG_MAX_SINKS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store sink
  naos_log_sinks[naos_log_sink_count] = sink;
  naos_log_sink_count++;

  // release mutex
  naos_unlock(naos_log_mutex);
}

void naos_log(const char *fmt, ...) {
  // format message
  va_list args;
  va_start(args, fmt);
  char msg[256];
  vsnprintf(msg, sizeof(msg), fmt, args);
  va_end(args);

  // dispatch to all sinks
  naos_lock(naos_log_mutex);
  size_t count = naos_log_sink_count;
  naos_unlock(naos_log_mutex);
  for (size_t i = 0; i < count; i++) {
    naos_log_sinks[i](msg);
  }

  // get device type
  const char *device_type = "unknown";
  if (naos_config() != NULL) {
    device_type = naos_config()->device_type;
  }

  // get log timestamp to synchronize with native logs
  uint32_t millis = esp_log_timestamp();

  // print message
  printf("N (%lu) %s: %s\n", millis, device_type, msg);
}
