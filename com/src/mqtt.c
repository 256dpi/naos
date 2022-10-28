#include <sdkconfig.h>
#include <stdlib.h>
#include <string.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>
#include <freertos/timers.h>
#include <esp_mqtt.h>

#include "com.h"
#include "net.h"
#include "utils.h"

static SemaphoreHandle_t naos_mqtt_mutex;
static char *naos_mqtt_base_topic_prefix = NULL;
static bool naos_mqtt_started = false;
static bool naos_mqtt_networked = false;

static char *naos_mqtt_with_base_topic(const char *topic) {
  // prefix base topic
  return naos_concat(naos_mqtt_base_topic_prefix, topic);
}

static naos_scope_t naos_mqtt_scope_from_topic(const char *topic) {
  // determine scope
  if (strncmp(topic, naos_mqtt_base_topic_prefix, strlen(naos_mqtt_base_topic_prefix)) == 0) {
    return NAOS_LOCAL;
  } else {
    return NAOS_GLOBAL;
  }
}

static const char *naos_mqtt_without_base_topic(const char *topic) {
  // return immediately if string is not prefixed with the base topic
  if (naos_mqtt_scope_from_topic(topic) == NAOS_GLOBAL) {
    return topic;
  }

  // return adjusted pointer
  return topic + strlen(naos_mqtt_base_topic_prefix);
}

static void naos_mqtt_status_handler(esp_mqtt_status_t status) {
  // acquire mutex
  NAOS_LOCK(naos_mqtt_mutex);

  // set status
  naos_mqtt_networked = status == ESP_MQTT_STATUS_CONNECTED;

  // release mutex
  NAOS_UNLOCK(naos_mqtt_mutex);
}

static void naos_mqtt_message_handler(const char *topic, uint8_t *payload, size_t len) {
  // get scope and scoped topic
  naos_scope_t scope = naos_mqtt_scope_from_topic(topic);
  const char *scoped_topic = naos_mqtt_without_base_topic(topic);

  // TODO: Set QoS and retained?

  // dispatch message
  naos_com_dispatch(scope, scoped_topic, payload, len, 0, false);
}

naos_com_status_t naos_mqtt_status() {
  // get status
  NAOS_LOCK(naos_mqtt_mutex);
  naos_com_status_t status = {
      .networked = naos_mqtt_networked,
  };
  NAOS_UNLOCK(naos_mqtt_mutex);

  return status;
}

static bool naos_mqtt_subscribe(naos_scope_t scope, const char *topic, int qos) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_mqtt_with_base_topic(topic);
  }

  // subscribe
  bool ret = esp_mqtt_subscribe(topic, qos);

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

static bool naos_mqtt_unsubscribe(naos_scope_t scope, const char *topic) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_mqtt_with_base_topic(topic);
  }

  // unsubscribe
  bool ret = esp_mqtt_unsubscribe(topic);

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

static bool naos_mqtt_publish(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                              bool retained) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_mqtt_with_base_topic(topic);
  }

  // publish
  bool ret = esp_mqtt_publish(topic, (uint8_t *)payload, len, qos, retained);

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

static void naos_mqtt_start() {
  // get settings
  const char *host = naos_get("mqtt-host");
  const char *port = naos_get("mqtt-port");
  const char *client_id = naos_get("mqtt-client-id");
  const char *username = naos_get("mqtt-username");
  const char *password = naos_get("mqtt-password");
  const char *base_topic = naos_get("mqtt-base-topic");

  // return if host is empty
  if (strlen(host) == 0) {
    return;
  }

  // acquire mutex
  NAOS_LOCK(naos_mqtt_mutex);

  // set flag
  naos_mqtt_started = true;

  // free base topic prefix if set
  if (naos_mqtt_base_topic_prefix != NULL) {
    free(naos_mqtt_base_topic_prefix);
  }

  // set base topic prefix
  naos_mqtt_base_topic_prefix = naos_concat(base_topic, "/");

  // release mutex
  NAOS_UNLOCK(naos_mqtt_mutex);

  // start the MQTT client
  esp_mqtt_start(host, port, client_id, username, password);
}

static void naos_mqtt_stop() {
  // stop the MQTT client
  esp_mqtt_stop();

  // acquire mutex
  NAOS_LOCK(naos_mqtt_mutex);

  // set flags
  naos_mqtt_started = false;
  naos_mqtt_networked = false;

  // release mutex
  NAOS_UNLOCK(naos_mqtt_mutex);
}

static void naos_mqtt_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_mqtt_configure");

  // restart MQTT if started
  if (naos_mqtt_started) {
    naos_mqtt_stop();
    naos_mqtt_start();
  }
}

static void naos_mqtt_manage() {
  // get network status
  bool connected = naos_net_connected();

  // handle status
  if (connected && !naos_mqtt_started) {
    naos_mqtt_start();
  } else if (!connected && naos_mqtt_started) {
    naos_mqtt_stop();
  }
}

static naos_param_t naos_mqtt_params[] = {
    {.name = "mqtt-host", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-port", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-client-id", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-username", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-password", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-base-topic", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "mqtt-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_mqtt_configure},
};

void naos_mqtt_init() {
  // create mutex
  naos_mqtt_mutex = xSemaphoreCreateMutex();

  // ensure base topic
  naos_mqtt_base_topic_prefix = strdup("");

  // initialize MQTT
  esp_mqtt_init(naos_mqtt_status_handler, naos_mqtt_message_handler, CONFIG_NAOS_MQTT_BUFFER_SIZE,
                CONFIG_NAOS_MQTT_COMMAND_TIMEOUT);

  // register parameters
  for (size_t i = 0; i < (sizeof(naos_mqtt_params) / sizeof(naos_mqtt_params[0])); i++) {
    naos_register(&naos_mqtt_params[i]);
  }

  // register transport
  naos_com_transport_t transport = {
      .status = naos_mqtt_status,
      .subscribe = naos_mqtt_subscribe,
      .unsubscribe = naos_mqtt_unsubscribe,
      .publish = naos_mqtt_publish,
  };
  naos_com_register(transport);

  // start manage timer
  TimerHandle_t timer = xTimerCreate("naos-mqtt", pdMS_TO_TICKS(1000), pdTRUE, 0, naos_mqtt_manage);
  xTimerStart(timer, 0);
}
