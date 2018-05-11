#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <naos.h>

static int counter = 0;

static char *message = NULL;

static char *var_s = NULL;
static int32_t var_l = 0;
static double var_d = 0;
static bool var_b = true;

static void ping() { naos_log("ping received!"); }

static void online() {
  // log info
  naos_log("online callback called");

  // subscribe to topic
  naos_subscribe("hello", 0, NAOS_LOCAL);
  naos_subscribe("fail", 0, NAOS_LOCAL);

  // clear and update message
  if (message != NULL) free(message);
  message = strdup(naos_get("message"));
}

static void update(const char *param, const char *value) {
  // log param change
  if (value != NULL) {
    naos_log("param %s updated to %s", param, value);
  } else {
    naos_log("param %s updated to NULL", param);
  }

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
  // check fail topic
  if (strcmp(topic, "fail") == 0 && scope == NAOS_LOCAL) {
    // cause error
    int r = 10 / 0;
    naos_log("error: %d", r);
  }

  // log other incoming message
  else {
    naos_log("%s message %s with payload %s (%ld) received", naos_scope_str(scope), topic, payload, len);
  }
}

static void loop() {
  // increment counter
  counter++;

  // log info
  naos_log("loop callback called (%d)", counter);

  // publish message
  naos_publish("hello", message, 0, false, NAOS_LOCAL);

  // save counter
  char buf[16];
  snprintf(buf, 16, "%d", counter);
  naos_set("counter", buf);
}

static float battery() { return 0.42; }

static void offline() {
  // log info
  naos_log("offline callback called");
}

static void status(naos_status_t status) {
  // log new status
  naos_log("status changed to %s", naos_status_str(status));
}

static naos_param_t params[] = {{.name = "var_s", .type = NAOS_STRING, .default_s = "", .sync_s = &var_s},
                                {.name = "var_l", .type = NAOS_LONG, .default_l = 0, .sync_l = &var_l},
                                {.name = "var_d", .type = NAOS_DOUBLE, .default_d = 0, .sync_d = &var_d},
                                {.name = "var_b", .type = NAOS_BOOL, .default_b = true, .sync_b = &var_b}};

static naos_config_t config = {.device_type = "naos-test",
                               .firmware_version = "0.0.1",
                               .parameters = params,
                               .num_parameters = 4,
                               .ping_callback = ping,
                               .online_callback = online,
                               .message_callback = handle,
                               .update_callback = update,
                               .loop_callback = loop,
                               .loop_interval = 1000,
                               .battery_level = battery,
                               .offline_callback = offline,
                               .status_callback = status};

void app_main() {
  // initialize naos
  naos_init(&config);
}
