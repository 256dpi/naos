#include <naos/sys.h>

#include <esp_log.h>
#include <esp_mqtt.h>
#include <stdlib.h>
#include <string.h>

#include "com.h"
#include "utils.h"
#include "system.h"

static naos_mutex_t naos_mqtt_mutex;
static bool naos_mqtt_started = false;
static bool naos_mqtt_networked = false;
static uint32_t naos_mqtt_generation = 0;

static struct {
  char *topic;
  char *payload;
  int qos;
  bool retained;
  naos_scope_t scope;
} naos_mqtt_lwt_config = {0};

static void naos_mqtt_status_handler(esp_mqtt_status_t status) {
  // acquire mutex
  naos_lock(naos_mqtt_mutex);

  // ignore callbacks if not started
  if (!naos_mqtt_started) {
    naos_unlock(naos_mqtt_mutex);
    return;
  }

  // set status
  bool networked = status == ESP_MQTT_STATUS_CONNECTED;
  if (naos_mqtt_networked != networked) {
    naos_mqtt_networked = networked;
    naos_mqtt_generation++;
  }

  // release mutex
  naos_unlock(naos_mqtt_mutex);
}

static void naos_mqtt_message_handler(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained) {
  // dispatch message
  naos_com_dispatch(topic, payload, len, qos, retained);
}

static naos_com_status_t naos_mqtt_status() {
  // get status
  naos_lock(naos_mqtt_mutex);
  naos_com_status_t status = {
      .networked = naos_mqtt_networked,
      .generation = naos_mqtt_generation,
  };
  naos_unlock(naos_mqtt_mutex);

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
  const char *host = naos_get_s("mqtt-host");
  const char *port = naos_get_s("mqtt-port");
  const bool tls = naos_get_b("mqtt-tls");
  const char *client_id = naos_get_s("mqtt-client-id");
  const char *username = naos_get_s("mqtt-username");
  const char *password = naos_get_s("mqtt-password");

  // return if host is empty
  if (strlen(host) == 0) {
    return;
  }

  // check and set flag
  naos_lock(naos_mqtt_mutex);
  if (naos_mqtt_started) {
    naos_unlock(naos_mqtt_mutex);
    return;
  }
  naos_mqtt_started = true;

  // resolve the last will, prefixing the base topic now as it may have changed
  // since the will was set
  char *lwt_topic = NULL;
  if (naos_mqtt_lwt_config.topic != NULL) {
    if (naos_mqtt_lwt_config.scope == NAOS_LOCAL) {
      lwt_topic = naos_com_with_base_topic(naos_mqtt_lwt_config.topic);
    } else {
      lwt_topic = strdup(naos_mqtt_lwt_config.topic);
    }
  }
  char *lwt_payload = naos_mqtt_lwt_config.payload != NULL ? strdup(naos_mqtt_lwt_config.payload) : NULL;
  int lwt_qos = naos_mqtt_lwt_config.qos;
  bool lwt_retained = naos_mqtt_lwt_config.retained;
  naos_unlock(naos_mqtt_mutex);

  // configure the last will outside the mutex, as the client invokes the
  // status handler while holding its own lock
  esp_mqtt_lwt(lwt_topic, lwt_payload, lwt_qos, lwt_retained);
  free(lwt_topic);
  free(lwt_payload);

  // start the MQTT client
  bool ok = esp_mqtt_tls(tls, true, NULL, 0);
  if (ok) {
    ok = esp_mqtt_start(host, port, client_id, username, password);
  }
  if (!ok) {
    ESP_LOGW(NAOS_LOG_TAG, "naos_mqtt_start: failed to start MQTT client");
    naos_lock(naos_mqtt_mutex);
    naos_mqtt_started = false;
    naos_mqtt_networked = false;
    naos_unlock(naos_mqtt_mutex);
  }
}

static void naos_mqtt_stop() {
  // check and clear flags
  naos_lock(naos_mqtt_mutex);
  if (!naos_mqtt_started) {
    naos_unlock(naos_mqtt_mutex);
    return;
  }
  naos_mqtt_started = false;
  naos_mqtt_networked = false;
  naos_unlock(naos_mqtt_mutex);

  // stop the MQTT client
  esp_mqtt_stop();
}

static void naos_mqtt_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_mqtt_configure");

  // stop and start MQTT
  naos_mqtt_stop();
  if (naos_status() >= NAOS_CONNECTED) {
    naos_mqtt_start();
  }
}

static void naos_mqtt_manage(naos_status_t status) {
  // get network status
  bool connected = status >= NAOS_CONNECTED;

  // get started
  naos_lock(naos_mqtt_mutex);
  bool started = naos_mqtt_started;
  naos_unlock(naos_mqtt_mutex);

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
    {.name = "mqtt-tls", .type = NAOS_BOOL, .mode = NAOS_SYSTEM},
    {.name = "mqtt-client-id", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-username", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-password", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_mqtt_configure},
};

void naos_mqtt_init(int core) {
  // create mutex
  naos_mqtt_mutex = naos_mutex();

  // initialize MQTT
  esp_mqtt_init(naos_mqtt_status_handler, naos_mqtt_message_handler, CONFIG_NAOS_MQTT_BUFFER_SIZE,
                CONFIG_NAOS_MQTT_COMMAND_TIMEOUT, core);

  // register parameters
  for (size_t i = 0; i < NAOS_COUNT(naos_mqtt_params); i++) {
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

void naos_mqtt_lwt(const char *topic, const char *payload, int qos, bool retained, naos_scope_t scope) {
  // acquire mutex
  naos_lock(naos_mqtt_mutex);

  // free previous values
  free(naos_mqtt_lwt_config.topic);
  free(naos_mqtt_lwt_config.payload);

  // store config, applied when the client is started next
  naos_mqtt_lwt_config.topic = topic != NULL ? strdup(topic) : NULL;
  naos_mqtt_lwt_config.payload = payload != NULL ? strdup(payload) : NULL;
  naos_mqtt_lwt_config.qos = qos;
  naos_mqtt_lwt_config.retained = retained;
  naos_mqtt_lwt_config.scope = scope;

  // release mutex
  naos_unlock(naos_mqtt_mutex);
}
