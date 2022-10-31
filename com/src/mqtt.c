#include <naos_sys.h>

#include <string.h>
#include <esp_mqtt.h>

#include "naos.h"
#include "com.h"
#include "net.h"
#include "utils.h"
#include "system.h"

static naos_mutex_t naos_mqtt_mutex;
static bool naos_mqtt_started = false;
static bool naos_mqtt_networked = false;
static uint32_t naos_mqtt_generation = false;

static void naos_mqtt_status_handler(esp_mqtt_status_t status) {
  // acquire mutex
  NAOS_LOCK(naos_mqtt_mutex);

  // set status
  naos_mqtt_networked = status == ESP_MQTT_STATUS_CONNECTED;
  if (naos_mqtt_networked) {
    naos_mqtt_generation++;
  }

  // release mutex
  NAOS_UNLOCK(naos_mqtt_mutex);
}

static void naos_mqtt_message_handler(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained) {
  // dispatch message
  naos_com_dispatch(topic, payload, len, qos, retained);
}

static naos_com_status_t naos_mqtt_status() {
  // get status
  NAOS_LOCK(naos_mqtt_mutex);
  naos_com_status_t status = {
      .networked = naos_mqtt_networked,
      .generation = naos_mqtt_generation,
  };
  NAOS_UNLOCK(naos_mqtt_mutex);

  return status;
}

static bool naos_mqtt_subscribe(const char *topic, int qos) {
  // subscribe topic
  return esp_mqtt_subscribe(topic, qos);
}

static bool naos_mqtt_unsubscribe(const char *topic) {
  // unsubscribe topic
  return esp_mqtt_unsubscribe(topic);
}

static bool naos_mqtt_publish(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained) {
  // publish message
  return esp_mqtt_publish(topic, payload, len, qos, retained);
}

static void naos_mqtt_start() {
  // get settings
  const char *host = naos_get("mqtt-host");
  const char *port = naos_get("mqtt-port");
  const char *client_id = naos_get("mqtt-client-id");
  const char *username = naos_get("mqtt-username");
  const char *password = naos_get("mqtt-password");

  // return if host is empty
  if (strlen(host) == 0) {
    return;
  }

  // set flag
  NAOS_LOCK(naos_mqtt_mutex);
  naos_mqtt_started = true;
  NAOS_UNLOCK(naos_mqtt_mutex);

  // start the MQTT client
  esp_mqtt_start(host, port, client_id, username, password);
}

static void naos_mqtt_stop() {
  // stop the MQTT client
  esp_mqtt_stop();

  // set flags
  NAOS_LOCK(naos_mqtt_mutex);
  naos_mqtt_started = false;
  naos_mqtt_networked = false;
  NAOS_UNLOCK(naos_mqtt_mutex);
}

static void naos_mqtt_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_mqtt_configure");

  // get started
  NAOS_LOCK(naos_mqtt_mutex);
  bool started = naos_mqtt_started;
  NAOS_UNLOCK(naos_mqtt_mutex);

  // restart MQTT if started
  if (started) {
    naos_mqtt_stop();
    naos_mqtt_start();
  }
}

static void naos_mqtt_manage(naos_status_t status) {
  // get network status
  bool connected = status >= NAOS_CONNECTED;

  // get started
  NAOS_LOCK(naos_mqtt_mutex);
  bool started = naos_mqtt_started;
  NAOS_UNLOCK(naos_mqtt_mutex);

  // handle status
  if (connected && !started) {
    naos_mqtt_start();
  } else if (!connected && started) {
    naos_mqtt_stop();
  }
}

static naos_param_t naos_mqtt_params[] = {
    {.name = "mqtt-host", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-port", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-client-id", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-username", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-password", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_mqtt_configure},
};

void naos_mqtt_init() {
  // create mutex
  naos_mqtt_mutex = naos_mutex();

  // initialize MQTT
  esp_mqtt_init(naos_mqtt_status_handler, naos_mqtt_message_handler, CONFIG_NAOS_MQTT_BUFFER_SIZE,
                CONFIG_NAOS_MQTT_COMMAND_TIMEOUT);

  // register parameters
  for (size_t i = 0; i < (sizeof(naos_mqtt_params) / sizeof(naos_mqtt_params[0])); i++) {
    naos_register(&naos_mqtt_params[i]);
  }

  // register transport
  naos_com_transport_t transport = {
      .name = "mqtt",
      .status = naos_mqtt_status,
      .subscribe = naos_mqtt_subscribe,
      .unsubscribe = naos_mqtt_unsubscribe,
      .publish = naos_mqtt_publish,
  };
  naos_com_register(transport);

  // handle status
  naos_system_subscribe(naos_mqtt_manage);
}
