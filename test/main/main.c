#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <naos.h>

static int counter = 0;

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

  // set counter
  if (strcmp(param, "counter") == 0) {
    counter = (int)strtol(value, NULL, 0);
  }

  // clear and update message
  if (strcmp(param, "message") == 0) {
    if (message != NULL) free(message);
    message = strdup(value);
  }
}

static void handle(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope) {
  // log incoming message
  naos_log("%s message %s with payload %s (%ld) received", naos_scope_str(scope), topic, payload, len);
}

static void loop() {
  // increment counter
  counter++;

  // log info
  naos_log("loop callback called (%d)", counter);

  // publish message
  naos_publish_str("hello", message, 0, false, NAOS_LOCAL);

  // save counter
  char buf[16];
  snprintf(buf, 16, "%d", counter);
  naos_set("counter", buf);
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

  // set message default
  naos_set("message", "world");
}
