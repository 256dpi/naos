#include <esp_err.h>
#include <sdkconfig.h>
#include <stdlib.h>
#include <string.h>

#include "mqtt.h"
#include "naos.h"
#include "utils.h"

static char *naos_mqtt_base_topic_prefix = NULL;

static naos_mqtt_message_callback_t naos_mqtt_message_callback = NULL;

// returned topics must be freed after use
static char *naos_mqtt_with_base_topic(const char *topic) {
  return naos_str_concat(naos_mqtt_base_topic_prefix, topic);
}

static naos_scope_t naos_mqtt_scope_from_topic(const char *topic) {
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

static void naos_mqtt_message_handler(const char *topic, uint8_t *payload, size_t len) {
  // remove base topic
  const char *un_prefixed_topic = naos_mqtt_without_base_topic(topic);

  // call callback
  naos_mqtt_message_callback(un_prefixed_topic, payload, len, naos_mqtt_scope_from_topic(topic));
}

void naos_mqtt_init(esp_mqtt_status_callback_t scb, naos_mqtt_message_callback_t mcb) {
  // save message callback
  naos_mqtt_message_callback = mcb;

  // call init
  esp_mqtt_init(scb, naos_mqtt_message_handler, CONFIG_NAOS_MQTT_BUFFER_SIZE, CONFIG_NAOS_MQTT_COMMAND_TIMEOUT);
}

void naos_mqtt_start(const char *host, unsigned int port, const char *client_id, const char *username,
                     const char *password, const char *base_topic) {
  // free base topic prefix if set
  if (naos_mqtt_base_topic_prefix != NULL) free(naos_mqtt_base_topic_prefix);

  // set base topic prefix
  naos_mqtt_base_topic_prefix = naos_str_concat(base_topic, "/");

  // start the mqtt process
  esp_mqtt_start(host, port, client_id, username, password);
}

bool naos_subscribe(const char *topic, int qos, naos_scope_t scope) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_mqtt_with_base_topic(topic);
  }

  // subscribe
  bool ret = esp_mqtt_subscribe(topic, qos);
  if (!ret && naos_config()->crash_on_mqtt_failures) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

bool naos_unsubscribe(const char *topic, naos_scope_t scope) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_mqtt_with_base_topic(topic);
  }

  // unsubscribe
  bool ret = esp_mqtt_unsubscribe(topic);
  if (!ret && naos_config()->crash_on_mqtt_failures) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

bool naos_publish(const char *topic, const char *payload, int qos, bool retained, naos_scope_t scope) {
  return naos_publish_r(topic, (char *)payload, strlen(payload), qos, retained, scope);
}

bool naos_publish_r(const char *topic, void *payload, size_t len, int qos, bool retained, naos_scope_t scope) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_mqtt_with_base_topic(topic);
  }

  // publish
  bool ret = esp_mqtt_publish(topic, payload, len, qos, retained);
  if (!ret && naos_config()->crash_on_mqtt_failures) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

void naos_mqtt_stop() {
  // stop the mqtt process
  esp_mqtt_stop();
}
