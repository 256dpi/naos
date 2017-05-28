#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <nadk.h>

static char *message = NULL;

static void online() {
  // print info
  printf("online\n");

  // subscribe to topic
  nadk_subscribe("hello", 0, NADK_LOCAL);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(nadk_get("message"));
}

static void update(const char *param, const char *value) {
  // print param change
  printf("param: %s=%s\n", param, value);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(value);
}

static void handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  // print incoming message
  printf("message: %s => %s (%d) [%d]\n", topic, payload, len, scope);
}

static void loop() {
  // publish message
  nadk_publish_str("hello", message, 0, false, NADK_LOCAL);
}

static void offline() {
  // print info
  printf("offline\n");
}

static void status(nadk_status_t status) {
  // print new status
  printf("status: %d\n", status);
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
