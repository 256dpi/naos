#include <naos.h>
#include <naos/ble.h>
#include <naos/wifi.h>
#include <naos/mqtt.h>
#include <naos/bridge.h>
#include <naos/debug.h>
#include <naos/serial.h>

extern const uint8_t foo_start[] asm("_binary_foo_txt_start");
extern const uint8_t foo_end[] asm("_binary_foo_txt_end");

static void crash() {
  int *ptr = NULL;
  *ptr = 42;
}

static naos_param_t params[] = {
    {.name = "foo", .type = NAOS_STRING, .default_s = ""},
    {.name = "baz", .type = NAOS_STRING, .default_s = ""},
    {.name = "crash", .type = NAOS_ACTION, .func_a = crash},
};

static naos_config_t config = {
    .parameters = params,
    .num_parameters = 3,
};

void app_main() {
  // initialize naos
  naos_init(&config);
  naos_ble_init((naos_ble_config_t){});
  naos_wifi_init();
  naos_mqtt_init(1);
  naos_mqtt_lwt("status", "offline", 0, true, NAOS_LOCAL);
  naos_serial_init_stdio(1);
  naos_bridge_install();
  naos_debug_install();

  // print foo
  naos_log("%d", foo_end - foo_start);
}
