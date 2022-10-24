#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <esp_eth.h>
#include <esp_eth_phy.h>
#include <esp_netif.h>

#include <naos.h>

#define ETHERNET false

static int counter = 0;

static char *message = NULL;

static char *var_s = NULL;
static int32_t var_l = 0;
static double var_d = 0;
static bool var_b = true;

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

static void fun_s(char *str) {
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

static void eth_init() {
  // prepare mac
  eth_mac_config_t mac_config = ETH_MAC_DEFAULT_CONFIG();
  mac_config.clock_config.rmii.clock_mode = EMAC_CLK_OUT;
  mac_config.clock_config.rmii.clock_gpio = EMAC_CLK_OUT_180_GPIO;
  esp_eth_mac_t *mac = esp_eth_mac_new_esp32(&mac_config);

  // prepare phy
  eth_phy_config_t phy_config = ETH_PHY_DEFAULT_CONFIG();
  phy_config.phy_addr = 0;
  esp_eth_phy_t *phy = esp_eth_phy_new_lan87xx(&phy_config);

  // install driver
  esp_eth_config_t config = ETH_DEFAULT_CONFIG(mac, phy);
  esp_eth_handle_t eth_handle = NULL;
  ESP_ERROR_CHECK(esp_eth_driver_install(&config, &eth_handle));

  // create interface
  esp_netif_config_t cfg = ESP_NETIF_DEFAULT_ETH();
  esp_netif_t *eth_netif = esp_netif_new(&cfg);

  // attach ethernet
  ESP_ERROR_CHECK(esp_netif_attach(eth_netif, esp_eth_new_netif_glue(eth_handle)));

  // start ethernet driver
  ESP_ERROR_CHECK(esp_eth_start(eth_handle));
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
};

static naos_config_t config = {
    .device_type = "naos-test",
    .firmware_version = "0.0.1",
    .parameters = params,
    .num_parameters = 8,
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

static naos_param_t param = {
    .name = "dyn_s",
    .type = NAOS_STRING,
};

void app_main() {
  // initialize naos
  naos_init(&config);

  // register parameter
  naos_register(&param);

  // initialize ethernet
  if (ETHERNET) {
    eth_init();
  }
}
