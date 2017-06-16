#include <stdlib.h>
#include <string.h>

#include <naos.h>

static char *message = NULL;

static void online() {
  // log info
  naos_log("online callback called");

  // subscribe to topic
  naos_subscribe("hello", 0, NAOS_LOCAL);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(naos_get("message"));
}

static void update(const char *param, const char *value) {
  // log param change
  naos_log("param %s updated to %s", param, value);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(value);
}

static void handle(const char *topic, const char *payload, unsigned int len, naos_scope_t scope) {
  // log incoming message
  naos_log("%s message %s with payload %s received", naos_scope_str(scope), topic, payload);
}

static void loop() {
  // log info
  naos_log("loop callback called");

  // publish message
  naos_publish_str("hello", message, 0, false, NAOS_LOCAL);
}

static void offline() {
  // log info
  naos_log("offline callback called");
}

static void status(naos_status_t status) {
  // log new status
  naos_log("status changed to %s", naos_status_str(status));
}

static naos_config_t config = {.device_type = "naos-test",
                               .firmware_version = "0.0.1",
                               .online_callback = online,
                               .message_callback = handle,
                               .update_callback = update,
                               .loop_callback = loop,
                               .loop_interval = 1000,
                               .offline_callback = offline,
                               .status_callback = status};

void app_main() {
  // initialize naos
  naos_init(&config);
}
