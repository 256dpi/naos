#include <stdlib.h>
#include <string.h>

#include <naos.h>
#include <naos_ble.h>
#include <naos_cpu.h>
#include <naos_wifi.h>
#include <naos_http.h>
#include <naos_eth.h>
#include <naos_mqtt.h>
#include <naos_manager.h>

#define ETHERNET false

static char *var_s = NULL;
static int32_t var_l = 0;
static double var_d = 0;
static bool var_b = true;

static int32_t counter = 0;

static void setup() {
  // log info
  naos_log("setup called!");
}

static void ping() {
  // log info
  naos_log("ping received!");
}

static void online() {
  // log info
  naos_log("online callback called");

  // subscribe to topic
  naos_subscribe("hello", 0, NAOS_LOCAL);
  naos_subscribe("fail", 0, NAOS_LOCAL);
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
}

static void handle(const char *topic, const uint8_t *payload, size_t len, naos_scope_t scope) {
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
  naos_set_l("counter", counter);

  // log info
  naos_log("loop callback called (%d)", counter);

  // publish message
  naos_publish("hello", naos_get("message"), 0, false, NAOS_LOCAL);
}

static float battery() {
  // return level
  return 0.42f;
}

static void offline() {
  // log info
  naos_log("offline callback called");
}

static void status(naos_status_t status) {
  // log new status
  naos_log("status changed to %s", naos_status_str(status));
}

static void fun_s(const char *str) {
  // log info
  naos_log("fun_s: %s", str);
}

static void fun_l(int32_t num) {
  // log info
  naos_log("fun_l: %d", num);
}

static void fun_d(double num) {
  // log info
  naos_log("fun_d: %f", num);
}

static void fun_b(bool ok) {
  // log info
  naos_log("fun_b: %s", ok ? "true" : "false");
}

static void fun_a() {
  // log info
  naos_log("fun_a: triggered");
}

static naos_param_t params[] = {
    {.name = "var_s", .type = NAOS_STRING, .default_s = "", .sync_s = &var_s},
    {.name = "var_l", .type = NAOS_LONG, .default_l = 0, .sync_l = &var_l},
    {.name = "var_d", .type = NAOS_DOUBLE, .default_d = 0, .sync_d = &var_d},
    {.name = "var_b", .type = NAOS_BOOL, .default_b = true, .sync_b = &var_b},
    {.name = "fun_s", .type = NAOS_STRING, .default_s = "", .func_s = fun_s},
    {.name = "fun_l", .type = NAOS_LONG, .default_l = 0, .func_l = &fun_l},
    {.name = "fun_d", .type = NAOS_DOUBLE, .default_d = 0, .func_d = &fun_d},
    {.name = "fun_b", .type = NAOS_BOOL, .default_b = true, .func_b = &fun_b},
    {.name = "fun_a", .type = NAOS_ACTION, .func_a = &fun_a},
};

static naos_config_t config = {
    .device_type = "naos-test",
    .device_version = "0.0.1",
    .parameters = params,
    .num_parameters = sizeof(params) / sizeof(params[0]),
    .setup_callback = setup,
    .ping_callback = ping,
    .online_callback = online,
    .message_callback = handle,
    .update_callback = update,
    .loop_callback = loop,
    .loop_interval = 1000,
    .battery_callback = battery,
    .offline_callback = offline,
    .status_callback = status,
    .password = "secret",
};

static naos_param_t param_counter = {
    .name = "counter",
    .type = NAOS_STRING,
    .mode = NAOS_VOLATILE,
};

static naos_param_t param_message = {
    .name = "message",
    .type = NAOS_STRING,
};

void app_main() {
  // initialize naos
  naos_init(&config);
  naos_cpu_init();
  naos_ble_init();
  naos_wifi_init();
  naos_http_init();
  naos_mqtt_init();
  naos_manager_init();
  if (ETHERNET) {
    naos_eth_olimex();
    naos_eth_init();
  }

  // register parameter
  naos_register(&param_counter);
  naos_register(&param_message);

  // initialize counter
  counter = naos_get_l("counter");
}
