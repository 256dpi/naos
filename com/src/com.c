#include <string.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>

#include "naos.h"
#include "com.h"
#include "utils.h"

#define NAOS_COM_MAX_TRANSPORTS 8
#define NAOS_COM_MAX_RECEIVERS 8

static SemaphoreHandle_t naos_com_mutex;
static naos_com_transport_t naos_com_transports[NAOS_COM_MAX_TRANSPORTS] = {0};
static size_t naos_com_transport_count = 0;
static naos_com_receiver_t naos_com_receivers[NAOS_COM_MAX_RECEIVERS] = {0};
static size_t naos_com_receiver_count = 0;
static naos_param_t *naos_com_base_topic = {0};

static char *naos_com_with_base_topic(const char *topic) {
  // prefix base topic
  return naos_format("%s/%s", naos_com_base_topic->value, topic);
}

static naos_scope_t naos_com_scope_from_topic(const char *topic) {
  // determine scope
  if (strncmp(topic, naos_com_base_topic->value, strlen(naos_com_base_topic->value)) == 0) {
    return NAOS_LOCAL;
  } else {
    return NAOS_GLOBAL;
  }
}

static const char *naos_com_without_base_topic(const char *topic) {
  // return immediately if string is not prefixed with the base topic
  if (naos_com_scope_from_topic(topic) == NAOS_GLOBAL) {
    return topic;
  }

  // return adjusted pointer
  return topic + strlen(naos_com_base_topic->value) + 1;
}

void naos_com_init() {
  // create mutex
  naos_com_mutex = xSemaphoreCreateMutex();

  // lookup base topic param
  naos_com_base_topic = naos_lookup("base-topic");
}

void naos_com_register(naos_com_transport_t transport) {
  // acquire mutex
  NAOS_LOCK(naos_com_mutex);

  // check count
  if (naos_com_transport_count >= NAOS_COM_MAX_TRANSPORTS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store transport
  naos_com_transports[naos_com_transport_count] = transport;
  naos_com_transport_count++;

  // release mutex
  NAOS_UNLOCK(naos_com_mutex);
}

void naos_com_subscribe(naos_com_receiver_t receiver) {
  // acquire mutex
  NAOS_LOCK(naos_com_mutex);

  // check count
  if (naos_com_receiver_count >= NAOS_COM_MAX_RECEIVERS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store transport
  naos_com_receivers[naos_com_receiver_count] = receiver;
  naos_com_receiver_count++;

  // release mutex
  NAOS_UNLOCK(naos_com_mutex);
}

void naos_com_dispatch(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained) {
  // get scope and scoped topic
  naos_scope_t scope = naos_com_scope_from_topic(topic);
  const char *scoped_topic = naos_com_without_base_topic(topic);

  // dispatch to all receivers
  NAOS_LOCK(naos_com_mutex);
  size_t count = naos_com_receiver_count;
  NAOS_UNLOCK(naos_com_mutex);
  for (size_t i = 0; i < count; i++) {
    naos_com_receivers[i](scope, scoped_topic, payload, len, qos, retained);
  }

  // call message callback if present
  if (naos_config()->message_callback != NULL) {
    naos_acquire();
    naos_config()->message_callback(scoped_topic, payload, len, scope);
    naos_release();
  }
}

bool naos_com_networked(uint32_t *generation) {
  // acquire mutex
  NAOS_LOCK(naos_com_mutex);

  // get status
  bool networked = false;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_status_t status = naos_com_transports[i].status();
    if (status.networked) {
      networked = true;
      if (generation != NULL) {
        *generation += (uint32_t)status.generation;
      }
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_com_mutex);

  return networked;
}

bool naos_subscribe(const char *topic, int qos, naos_scope_t scope) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_com_with_base_topic(topic);
  }

  // subscribe to all transports
  bool ok = true;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_transport_t transport = naos_com_transports[i];
    if (transport.subscribe != NULL) {
      if (!transport.subscribe(topic, qos)) {
        ok = false;
      }
    }
  }

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ok;
}

bool naos_unsubscribe(const char *topic, naos_scope_t scope) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_com_with_base_topic(topic);
  }

  // unsubscribe to all transports
  bool ok = true;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_transport_t transport = naos_com_transports[i];
    if (transport.unsubscribe != NULL) {
      if (!transport.unsubscribe(topic)) {
        ok = false;
      }
    }
  }

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ok;
}

bool naos_publish(const char *topic, const char *payload, int qos, bool retained, naos_scope_t scope) {
  return naos_publish_r(topic, (char *)payload, strlen(payload), qos, retained, scope);
}

bool naos_publish_b(const char *topic, bool payload, int qos, bool retained, naos_scope_t scope) {
  return naos_publish(topic, naos_i2str(payload), qos, retained, scope);
}

bool naos_publish_l(const char *topic, int32_t payload, int qos, bool retained, naos_scope_t scope) {
  return naos_publish(topic, naos_i2str(payload), qos, retained, scope);
}

bool naos_publish_d(const char *topic, double payload, int qos, bool retained, naos_scope_t scope) {
  return naos_publish(topic, naos_d2str(payload), qos, retained, scope);
}

bool naos_publish_r(const char *topic, void *payload, size_t len, int qos, bool retained, naos_scope_t scope) {
  // add base topic if scope is local
  if (scope == NAOS_LOCAL) {
    topic = naos_com_with_base_topic(topic);
  }

  // publish to all transports
  bool ok = true;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_transport_t transport = naos_com_transports[i];
    if (transport.publish != NULL) {
      if (!transport.publish(topic, payload, len, qos, retained)) {
        ok = false;
      }
    }
  }

  // free prefixed topic
  if (scope == NAOS_LOCAL) {
    free((void *)topic);
  }

  return ok;
}
