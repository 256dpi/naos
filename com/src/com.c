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

void naos_com_init() {
  // create mutex
  naos_com_mutex = xSemaphoreCreateMutex();
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

void naos_com_dispatch(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                       bool retained) {
  // dispatch to all receivers
  NAOS_LOCK(naos_com_mutex);
  size_t count = naos_com_receiver_count;
  NAOS_UNLOCK(naos_com_mutex);
  for (size_t i = 0; i < count; i++) {
    naos_com_receivers[i](scope, topic, payload, len, qos, retained);
  }

  // call message callback if present
  if (naos_config()->message_callback != NULL) {
    naos_acquire();
    naos_config()->message_callback(topic, payload, len, scope);
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
  // subscribe to all transports
  bool ok = true;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_transport_t transport = naos_com_transports[i];
    if (transport.subscribe != NULL) {
      if (!transport.subscribe(scope, topic, qos)) {
        ok = false;
      }
    }
  }

  return ok;
}

bool naos_unsubscribe(const char *topic, naos_scope_t scope) {
  // unsubscribe to all transports
  bool ok = true;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_transport_t transport = naos_com_transports[i];
    if (transport.unsubscribe != NULL) {
      if (!transport.unsubscribe(scope, topic)) {
        ok = false;
      }
    }
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
  // publish to all transports
  bool ok = true;
  for (size_t i = 0; i < naos_com_transport_count; i++) {
    naos_com_transport_t transport = naos_com_transports[i];
    if (transport.publish != NULL) {
      if (!transport.publish(scope, topic, payload, len, qos, retained)) {
        ok = false;
      }
    }
  }

  return ok;
}
