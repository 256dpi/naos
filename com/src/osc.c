#include <naos_osc.h>
#include <naos_sys.h>

#include "naos.h"
#include "com.h"
#include "utils.h"
#include "net.h"

#define NAOS_OSC_MAX_TARGETS 8

naos_mutex_t naos_osc_mutex;
esp_osc_client_t naos_osc_client;
esp_osc_target_t naos_osc_targets[NAOS_OSC_MAX_TARGETS];
size_t naos_osc_target_count = 0;
esp_osc_callback_t naos_osc_filter_callback = NULL;

static bool naos_osc_callback(const char *topic, const char *format, esp_osc_value_t *values) {
  // filter using callback if available
  if (naos_osc_filter_callback != NULL) {
    if (!naos_osc_filter_callback(topic, format, values)) {
      return true;
    }
  }

  // skip unsupported format
  if (strcmp(format, "b") != 0) {
    ESP_LOGI(NAOS_LOG_TAG, "naos_osc_callback: skipping unsupported format (%s)", format);
    return true;
  }

  // dispatch message
  naos_com_dispatch(topic, (const uint8_t *)values[0].b, values[0].bl, 0, false);

  return true;
}

static void naos_osc_task() {
  for (;;) {
    // receive messages
    esp_osc_receive(&naos_osc_client, naos_osc_callback);
  }
}

static naos_com_status_t naos_osc_status() {
  // prepare status
  naos_com_status_t status = {
      .networked = naos_net_connected(NULL),
      .generation = 1,
  };

  return status;
}

static bool naos_osc_publish(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained) {
  // send message
  return naos_osc_send(topic, "b", (int)len, payload);
}

static void naos_osc_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_osc_configure");

  // acquire mutex
  NAOS_LOCK(naos_osc_mutex);

  // get port
  int32_t port = naos_get_l("osc-port");

  // parse targets
  naos_osc_target_count = 0;
  char *targets = strdup(naos_get("osc-targets"));
  char *target = strtok(targets, ",");
  while (target != NULL && naos_osc_target_count < NAOS_OSC_MAX_TARGETS) {
    char target_addr[16];
    int target_port;
    if (sscanf(target, "%15[^:]:%d", target_addr, &target_port) == 2) {
      naos_osc_targets[naos_osc_target_count] = esp_osc_target(target_addr, target_port);
      naos_osc_target_count++;
    }
    target = strtok(NULL, ",");
  }
  free(targets);

  // start the OSC client
  esp_osc_init(&naos_osc_client, CONFIG_NAOS_OSC_BUFFER_SIZE, port);

  // release mutex
  NAOS_UNLOCK(naos_osc_mutex);
}

static naos_param_t naos_osc_params[] = {
    {.name = "osc-port", .type = NAOS_LONG, .mode = NAOS_SYSTEM},
    {.name = "osc-targets", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "osc-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_osc_configure},
};

void naos_osc_init() {
  // create mutex
  naos_osc_mutex = naos_mutex();

  // register parameters
  for (size_t i = 0; i < (sizeof(naos_osc_params) / sizeof(naos_osc_params[0])); i++) {
    naos_register(&naos_osc_params[i]);
  }

  // register transport
  naos_com_transport_t transport = {
      .name = "osc",
      .status = naos_osc_status,
      .publish = naos_osc_publish,
  };
  naos_com_register(transport);

  // perform initial configuration
  naos_osc_configure();

  // run task
  naos_run("naos-osc", 4096, naos_osc_task);
}

void naos_osc_filter(esp_osc_callback_t filter) {
  // acquire mutex
  NAOS_LOCK(naos_osc_mutex);

  // set filter
  naos_osc_filter_callback = filter;

  // release mutex
  NAOS_UNLOCK(naos_osc_mutex);
}

bool naos_osc_send(const char *topic, const char *format, ...) {
  // acquire mutex
  NAOS_LOCK(naos_osc_mutex);

  // send message
  va_list args;
  va_start(args, format);
  bool ok = true;
  for (size_t i = 0; i < naos_osc_target_count; i++) {
    if (!esp_osc_send_v(&naos_osc_client, &naos_osc_targets[i], topic, format, args)) {
      ok = false;
    }
  }
  va_end(args);

  // release mutex
  NAOS_UNLOCK(naos_osc_mutex);

  return ok;
}
