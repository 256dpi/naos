#include <stdlib.h>
#include <string.h>

#include <nadk.h>

static char *message = NULL;

static void online() {
  // log info
  nadk_log("online");

  // subscribe to topic
  nadk_subscribe("hello", 0, NADK_LOCAL);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(nadk_get("message"));
}

static void update(const char *param, const char *value) {
  // log param change
  nadk_log("param: %s=%s", param, value);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(value);
}

static void handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // log incoming message
  nadk_log("message: %s => %s (%d) [%d]", topic, payload, len, scope);
}

static void loop() {
  // publish message
  nadk_publish_str("hello", message, 0, false, NADK_LOCAL);
}

static void offline() {
  // log info
  nadk_log("offline");
}

static void status(nadk_status_t status) {
  // log new status
  nadk_log("status: %d", status);
}

static nadk_config_t config = {.device_type = "nadk-test",
                               .firmware_version = "0.0.1",
                               .online_callback = online,
                               .message_callback = handle,
                               .update_callback = update,
                               .loop_callback = loop,
                               .loop_interval = 1000,
                               .offline_callback = offline,
                               .status_callback = status};

void app_main() {
  // initialize nadk
  nadk_init(&config);
}
