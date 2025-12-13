#include <naos/msg.h>
#include <naos/sys.h>

#include <stdio.h>
#include <string.h>

#include "system.h"
#include "com.h"

static uint8_t naos_bridge_channel = 0;

static void naos_bridge_status(naos_status_t status) {
  // check status
  if (status != NAOS_NETWORKED) {
    return;
  }

  // subscribe topics
  naos_subscribe("naos/discover", 0, NAOS_GLOBAL);
  naos_subscribe("naos/inbox", 0, NAOS_LOCAL);
}

static void naos_bridge_discover() {
  // get info
  const char *app_name = naos_config()->app_name;
  const char *app_version = naos_config()->app_version;
  const char *device_name = naos_get_s("device-name");
  const char *base_topic = naos_get_s("base-topic");

  // construct description
  char loc[96];
  snprintf(loc, sizeof(loc), "0|%s|%s|%s|%s", app_name, app_version, device_name, base_topic);

  // publish description
  naos_publish_s("naos/describe", loc, 0, false, NAOS_GLOBAL);
}

static void naos_bridge_handler(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                bool retained) {
  // handle discover messages
  if (scope == NAOS_GLOBAL && strcmp(topic, "naos/discover") == 0) {
    naos_defer("bridge", 0, naos_bridge_discover);
  }

  // dispatch incoming messages
  if (scope == NAOS_LOCAL && strcmp(topic, "naos/inbox") == 0) {
    naos_msg_dispatch(naos_bridge_channel, (uint8_t *)payload, len, NULL);
  }
}

static uint16_t naos_bridge_mtu() { return 4096; }

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
  naos_bridge_channel = naos_msg_register((naos_msg_channel_t){
      .name = "bridge",
      .mtu = naos_bridge_mtu,
      .send = naos_bridge_send,
  });
}
