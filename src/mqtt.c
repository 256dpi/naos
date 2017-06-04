#include <sdkconfig.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#include "general.h"
#include "mqtt.h"

static char *nadk_mqtt_base_topic_prefix = NULL;

static nadk_mqtt_message_callback_t nadk_mqtt_message_callback = NULL;

// returned topics must be freed after use
static char *nadk_mqtt_with_base_topic(const char *topic) {
  return nadk_str_concat(nadk_mqtt_base_topic_prefix, topic);
}

static nadk_scope_t nadk_mqtt_scope_from_topic(const char *topic) {
  if (strncmp(topic, nadk_mqtt_base_topic_prefix, strlen(nadk_mqtt_base_topic_prefix)) == 0) {
    return NADK_LOCAL;
  } else {
    return NADK_GLOBAL;
  }
}

static const char *nadk_mqtt_without_base_topic(const char *topic) {
  // return immediately if string is not prefixed with the base topic
  if (nadk_mqtt_scope_from_topic(topic) == NADK_GLOBAL) {
    return topic;
  }

  // return adjusted pointer
  return topic + strlen(nadk_mqtt_base_topic_prefix);
}

static void nadk_mqtt_message_handler(const char *topic, const char *payload, unsigned int len) {
  // remove base topic
  const char *un_prefixed_topic = nadk_mqtt_without_base_topic(topic);

  // call callback
  nadk_mqtt_message_callback(un_prefixed_topic, payload, len, nadk_mqtt_scope_from_topic(topic));
}

void nadk_mqtt_init(esp_mqtt_status_callback_t scb, nadk_mqtt_message_callback_t mcb) {
  // save message callback
  nadk_mqtt_message_callback = mcb;

  // call init
  esp_mqtt_init(scb, nadk_mqtt_message_handler, CONFIG_NADK_MQTT_BUFFER_SIZE, CONFIG_NADK_MQTT_COMMAND_TIMEOUT);
}

void nadk_mqtt_start(const char *host, unsigned int port, const char *client_id, const char *username,
                     const char *password, const char *base_topic) {
  // free base topic prefix if set
  if (nadk_mqtt_base_topic_prefix != NULL) free(nadk_mqtt_base_topic_prefix);

  // set base topic prefix
  nadk_mqtt_base_topic_prefix = nadk_str_concat(base_topic, "/");

  // start the mqtt process
  esp_mqtt_start(host, port, client_id, username, password);
}

bool nadk_subscribe(const char *topic, int qos, nadk_scope_t scope) {
  // add base topic if scope is local
  if (scope == NADK_LOCAL) {
    topic = nadk_mqtt_with_base_topic(topic);
  }

  // subscribe
  bool ret = esp_mqtt_subscribe(topic, qos);

  // free prefixed topic
  if (scope == NADK_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

bool nadk_unsubscribe(const char *topic, nadk_scope_t scope) {
  // add base topic if scope is local
  if (scope == NADK_LOCAL) {
    topic = nadk_mqtt_with_base_topic(topic);
  }

  // unsubscribe
  bool ret = esp_mqtt_unsubscribe(topic);

  // free prefixed topic
  if (scope == NADK_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

bool nadk_publish(const char *topic, void *payload, uint16_t len, int qos, bool retained, nadk_scope_t scope) {
  // add base topic if scope is local
  if (scope == NADK_LOCAL) {
    topic = nadk_mqtt_with_base_topic(topic);
  }

  // publish
  bool ret = esp_mqtt_publish(topic, payload, len, qos, retained);

  // free prefixed topic
  if (scope == NADK_LOCAL) {
    free((void *)topic);
  }

  return ret;
}

bool nadk_publish_str(const char *topic, const char *str, int qos, bool retained, nadk_scope_t scope) {
  return nadk_publish(topic, (char *)str, (uint16_t)strlen(str), qos, retained, scope);
}

bool nadk_publish_int(const char *topic, int num, int qos, bool retained, nadk_scope_t scope) {
  char buf[33];
  itoa(num, buf, 10);

  return nadk_publish_str(topic, buf, qos, retained, scope);
}

void nadk_mqtt_stop() {
  // stop the mqtt process
  esp_mqtt_stop();
}
