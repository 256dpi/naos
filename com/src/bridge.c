#include <naos/msg.h>

#include <string.h>

#include "system.h"
#include "com.h"

static uint8_t naos_bridge_channel = 0;

static void naos_bridge_status(naos_status_t status) {
  // check status
  if (status != NAOS_NETWORKED) {
    return;
  }

  // subscribe topic
  naos_subscribe("naos/inbox", 0, NAOS_LOCAL);
}

static void naos_bridge_handler(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                bool retained) {
  // check scope and topics
  if (scope != NAOS_LOCAL || strcmp(topic, "naos/inbox") != 0) {
    return;
  }

  // dispatch message
  naos_msg_channel_dispatch(naos_bridge_channel, (uint8_t *)payload, len, NULL);
}

static bool naos_bridge_send(const uint8_t *data, size_t len, void *ctx) {
  // publish message
  bool ok = naos_publish("naos/outbox", (void *)data, len, 0, false, NAOS_LOCAL);

  return ok;
}

void naos_bridge_install() {
  // subscribe status
  naos_system_subscribe(naos_bridge_status);

  // subscribe handler
  naos_com_subscribe(naos_bridge_handler);

  // register channel
  naos_bridge_channel = naos_msg_channel_register((naos_msg_channel_t){
      .name = "bridge",
      .mtu = 4096,
      .send = naos_bridge_send,
  });
}
