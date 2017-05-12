#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#include <esp_mqtt.h>

#include "mqtt.h"

static char *nadk_mqtt_base_topic = NULL;

static char nadk_mqtt_topic_cache[ESP_MQTT_BUFFER_SIZE];

static esp_mqtt_message_callback_t nadk_mqtt_message_callback = NULL;

static const char *nadk_mqtt_with_base_topic(const char *topic) {
  // check base topic
  if (nadk_mqtt_base_topic == NULL) {
    return topic;
  }

  // write base topic into cache
  strcpy(nadk_mqtt_topic_cache, nadk_mqtt_base_topic);

  // write topic into cache
  strcpy(nadk_mqtt_topic_cache + strlen(nadk_mqtt_base_topic), topic);

  // return pointer to prefixed topic
  return nadk_mqtt_topic_cache;
}

static const char *nadk_mqtt_without_base_topic(const char *topic) {
  // check base topic
  if (nadk_mqtt_base_topic == NULL) {
    return topic;
  }

  // return immediately if string is not prefixed with the base topic
  if (strncmp(topic, nadk_mqtt_base_topic, strlen(nadk_mqtt_base_topic)) != 0) {
    return topic;
  }

  // return adjusted pointer
  return topic + strlen(nadk_mqtt_base_topic);
}

static void nadk_mqtt_message_handler(const char *topic, const char *payload, unsigned int len) {
  // remove base topic
  const char *un_prefixed_topic = nadk_mqtt_without_base_topic(topic);

  // call callback
  nadk_mqtt_message_callback(un_prefixed_topic, payload, len);
}

void nadk_mqtt_init(esp_mqtt_status_callback_t scb, esp_mqtt_message_callback_t mcb) {
  // save message callback
  nadk_mqtt_message_callback = mcb;

  // call init
  esp_mqtt_init(scb, nadk_mqtt_message_handler);
}

void nadk_mqtt_start(const char *host, unsigned int port, const char *client_id, const char *username,
                     const char *password, const char *base_topic) {
  // free base topic if set
  if (nadk_mqtt_base_topic != NULL) {
    free(nadk_mqtt_base_topic);
    nadk_mqtt_base_topic = NULL;
  }

  // set base topic if provided
  if (base_topic != NULL) {
    nadk_mqtt_base_topic = strdup(base_topic);
  }

  esp_mqtt_start(host, port, client_id, username, password);
}

bool nadk_subscribe(const char *topic, int qos) {
  // add base topic
  const char *prefixed_topic = nadk_mqtt_with_base_topic(topic);

  return esp_mqtt_subscribe(prefixed_topic, qos);
}

bool nadk_unsubscribe(const char *topic) {
  // add base topic
  const char *prefixed_topic = nadk_mqtt_with_base_topic(topic);

  return esp_mqtt_unsubscribe(prefixed_topic);
}

bool nadk_publish(const char *topic, void *payload, uint16_t len, int qos, bool retained) {
  // add base topic
  const char *prefixed_topic = nadk_mqtt_with_base_topic(topic);

  return esp_mqtt_publish(prefixed_topic, payload, len, qos, retained);
}

bool nadk_publish_str(const char *topic, const char *payload, int qos, bool retained) {
  return nadk_publish(topic, (char *)payload, (uint16_t)strlen(payload), qos, retained);
}

void nadk_mqtt_stop() { esp_mqtt_stop(); }
