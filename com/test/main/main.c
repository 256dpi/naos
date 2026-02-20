#include <stdlib.h>
#include <string.h>
#include <math.h>

#include <naos.h>
#include <naos/ble.h>
#include <naos/cpu.h>
#include <naos/wifi.h>
#include <naos/http.h>
#include <naos/eth.h>
#include <naos/mqtt.h>
#include <naos/osc.h>
#include <naos/bridge.h>
#include <naos/fs.h>
#include <naos/serial.h>
#include <naos/relay.h>
#include <naos/mdns.h>
#include <naos/metrics.h>
#include <naos/connect.h>
#include <naos/auth.h>
#include <naos/debug.h>
#include <naos/sys.h>

#define ETHERNET false

static char *var_s = NULL;
static int32_t var_l = 0;
static double var_d = 0;
static bool var_b = true;

static int32_t counter = 0;
static double gauge[2][2] = {0};

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

static void update(naos_param_t *param) {
  // log param change
  if (param->type == NAOS_ACTION) {
    naos_log("param %s triggered", param->name);
  } else {
    naos_log("param %s updated to '%s'", param->name, param->current.buf);
  }

  // set counter
  if (strcmp(param->name, "counter") == 0) {
    counter = (int)strtol((const char *)param->current.buf, NULL, 0);
  }
}

static void message(const char *topic, const uint8_t *payload, size_t len, naos_scope_t scope) {
  // skip system messages
  if (strncmp(topic, "naos/", 5) == 0) {
    return;
  }

  // log other incoming message
  naos_log("%s message at %s with payload %s (%ld) received", naos_scope_str(scope), topic, payload, len);
}

static void loop() {
  // increment counter
  counter++;
  naos_set_l("counter", counter);

  // log info
  naos_log("loop callback called (%d)", counter);

  // publish message
  naos_publish_s("hello", naos_get_s("message"), 0, false, NAOS_LOCAL);

  // send osc message
  naos_osc_send("counter", "i", counter);
}

static void offline() {
  // log info
  naos_log("offline callback called");
}

static void status(naos_status_t status) {
  // log new status
  naos_log("status changed to %s", naos_status_str(status));
}

static float battery() {
  // return level
  return 0.42f;
}

static void fun_s(const char *str) {
  // handle fail
  if (strcmp(str, "fail") == 0) {
    ESP_ERROR_CHECK(ESP_FAIL);
    return;
  }

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

static bool osc_filter(const char *topic, const char *format, esp_osc_value_t *values) {
  // handle counter
  if (strcmp(topic, "counter") == 0 && strcmp(format, "i") == 0) {
    naos_log("osc_filter: %i", values[0].i);
    return false;
  }

  return true;
}

static uint64_t host_scan() {
  // enumerate one device
  return 0b1;
}

static bool host_to_device(uint8_t num, uint8_t *data, size_t len, naos_relay_meta_t meta) {
  // relay message
  naos_relay_device_process(data, len, meta);

  return true;
}

static bool device_to_host(uint8_t *data, size_t len) {
  // relay message
  naos_relay_host_process(0, data, len);

  return true;
}

static void auth_test() {
  // check status
  bool provisioned = naos_auth_status();
  naos_log("auth_test: status: %s", provisioned ? "provisioned" : "not provisioned");

  // test provisioning
  if (!provisioned) {
    // provision key and data
    const char *key = "0123456789abcdef0123456789abcdef";
    naos_auth_data_t data = {
        .version = 1,
        .uuid = {0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
        .product = 0x1,
        .revision = 0x2,
        .batch = 0x3,
        .date = 0x4,
    };
    naos_auth_err_t err = naos_auth_provision((uint8_t *)key, &data);
    if (err != NAOS_AUTH_ERR_OK) {
      naos_log("auth_test: provisioning failed: %d", err);
      return;
    }

    // check status again
    provisioned = naos_auth_status();
    naos_log("auth_test: status: %s", provisioned ? "provisioned" : "not provisioned");
  }

  // describe device
  naos_auth_data_t data2;
  naos_auth_err_t err = naos_auth_describe(&data2);
  if (err != NAOS_AUTH_ERR_OK) {
    naos_log("auth_test: description failed: %d", err);
    return;
  }

  // print description
  naos_log("auth_test: description: version=%d", data2.version);
  printf("  uuid=");
  for (int i = 0; i < 16; i++) {
    printf("%02x", data2.uuid[i]);
  }
  printf("\n  product=%d\n", data2.product);
  printf("  revision=%d\n", data2.revision);
  printf("  batch=%d\n", data2.batch);
  printf("  date=%lu\n", data2.date);

  // test attestation
  const char *input = "challenge";
  uint8_t output[32] = {0};
  err = naos_auth_attest(input, strlen(input), output);
  if (err != NAOS_AUTH_ERR_OK) {
    naos_log("auth_test: verification failed: %d", err);
    return;
  }

  // check output
  uint8_t expected[32] = {0xee, 0x3b, 0x6e, 0x31, 0x50, 0xb5, 0xb7, 0x00, 0x70, 0x47, 0x27,
                          0x80, 0xc1, 0x98, 0x8f, 0xe5, 0x69, 0xee, 0xcc, 0x24, 0x93, 0x24,
                          0x7c, 0x7a, 0x1a, 0xb5, 0x1e, 0xfd, 0x26, 0x3d, 0x5b, 0x77};
  if (memcmp(output, expected, sizeof(expected)) != 0) {
    naos_log("auth_test: verification failed: output does not match expected");
    return;
  }

  // print output
  for (int i = 0; i < 32; i++) {
    printf("%02x", output[i]);
  }
  printf("\n");
}

void ble_pairing_test() {
  // clear allowlist
  naos_log("clearing allowlist: %d", naos_ble_allowlist_length());
  naos_ble_allowlist_clear();

  // clear peerlist
  naos_log("clearing bonding list: %d", naos_ble_peerlist_length());
  naos_ble_peerlist_clear();

  for (;;) {
    // enable pairing
    naos_ble_enable_pairing();
    naos_log("enabled pairing");

    // wait for connection
    naos_log("awaiting connection...");
    if (naos_ble_await(10000)) {
      naos_log("got connection!");
    }

    // disable pairing
    naos_ble_disable_pairing();
    naos_log("disabled pairing");
    naos_delay(20000);
  }
}

static naos_param_t params[] = {
    {.name = "var_s", .type = NAOS_STRING, .default_s = "", .sync_s = &var_s},
    {.name = "var_l", .type = NAOS_LONG, .default_l = 0, .sync_l = &var_l},
    {.name = "var_d", .type = NAOS_DOUBLE, .default_d = 0, .sync_d = &var_d},
    {.name = "var_b", .type = NAOS_BOOL, .default_b = true, .sync_b = &var_b},
    {.name = "fun_s", .type = NAOS_STRING, .default_s = "", .func_s = fun_s, .skip_func_init = true},
    {.name = "fun_l", .type = NAOS_LONG, .default_l = 0, .func_l = fun_l, .skip_func_init = true},
    {.name = "fun_d", .type = NAOS_DOUBLE, .default_d = 0, .func_d = fun_d, .skip_func_init = true},
    {.name = "fun_b", .type = NAOS_BOOL, .default_b = true, .func_b = fun_b, .skip_func_init = true},
    {.name = "fun_a", .type = NAOS_ACTION, .func_a = fun_a},
    {.name = "reset", .type = NAOS_ACTION, .func_a = naos_reset},
};

static naos_config_t config = {
    .default_password = "",
    .parameters = params,
    .num_parameters = sizeof(params) / sizeof(params[0]),
    .setup_callback = setup,
    .ping_callback = ping,
    .online_callback = online,
    .update_callback = update,
    .message_callback = message,
    .loop_callback = loop,
    .loop_interval = 1000,
    .offline_callback = offline,
    .status_callback = status,
    .battery_callback = battery,
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

static naos_metric_t counter_metric = {
    .name = "counter",
    .kind = NAOS_METRIC_COUNTER,
    .type = NAOS_METRIC_LONG,
    .data = &counter,
};

static naos_metric_t gauge_metric = {
    .name = "gauge",
    .kind = NAOS_METRIC_GAUGE,
    .type = NAOS_METRIC_DOUBLE,
    .data = gauge,
    .keys = {"a", "b"},
    .values = {"a1", "a2", NULL, "b1", "b2"},
};

static void gauge_task() {
  for (;;) {
    for (int i = 0; i < 10; i++) {
      // calculate sinuses
      gauge[0][0] = sin(i * 0.1);
      gauge[0][1] = cos(i * 0.1);
      gauge[1][0] = tan(i * 0.1);
      gauge[1][1] = atan(i * 0.1);

      // delay
      naos_delay(100);
    }
  }
}

void app_main() {
  // initialize naos
  naos_init(&config);
  naos_cpu_init();
  naos_ble_init((naos_ble_config_t){
      .pairing = false,
      .bonding = false,
  });
  naos_wifi_init();
  naos_http_init(1);
  naos_http_serve_str("/", "text/html", "<h1>Hello world!</h1>");
  naos_http_serve_str("/foo", "text/css", "body { color: red; }");
  naos_mqtt_init(1);
  naos_osc_init(1);
  naos_osc_filter(osc_filter);
  naos_serial_init_stdio();
  if (CONFIG_SOC_USB_SERIAL_JTAG_SUPPORTED) {
    naos_serial_init_secio();
  }
  naos_mdns_init((naos_mdns_config_t){
      .main = true,
      .http = true,
      .osc = true,
  });
  if (ETHERNET) {
    naos_eth_olimex();
    // naos_eth_w5500((naos_eth_w5500_t){});
    naos_eth_init();
  }

  // print app name and version
  naos_log("app name: %s", config.app_name);
  naos_log("app version: %s", config.app_version);

  // install bridge channel
  naos_bridge_install();

  // mount FAT file system
  naos_fs_mount_fat("/data", "storage", 5);

  // install file system endpoint
  naos_fs_install((naos_fs_config_t){
      .root = "/data",
  });

  // initialize relay
  naos_relay_host_init((naos_relay_host_t){
      .scan = host_scan,
      .send = host_to_device,
  });
  naos_relay_device_init((naos_relay_device_t){
      .mtu = 2048,
      .send = device_to_host,
  });

  // register parameters
  naos_register(&param_counter);
  naos_register(&param_message);

  // add metrics
  naos_metrics_add(&counter_metric);
  naos_metrics_add(&gauge_metric);

  // initialize connect
  naos_connect_init();

  // initialize auth
  naos_auth_install();

  // initialize debug
  naos_debug_install();

  // initialize counter
  counter = naos_get_l("counter");

  // run metric task
  naos_run("metrics", 4096, 1, gauge_task);

  // start
  // naos_start();

  // test authentication
  // auth_test();

  // test BLE pairing
  // ble_pairing_test();
}
