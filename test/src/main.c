#include <naos.h>
#include <naos/ble.h>
#include <naos/wifi.h>
#include <naos/mqtt.h>
#include <naos/manager.h>

extern const uint8_t foo_start[] asm("_binary_foo_txt_start");
extern const uint8_t foo_end[] asm("_binary_foo_txt_end");

static naos_param_t params[] = {
    {.name = "foo", .type = NAOS_STRING, .default_s = ""},
    {.name = "baz", .type = NAOS_STRING, .default_s = ""},
};

static naos_config_t config = {
    .app_name = "foo",
    .app_version = "0.1.0",
    .parameters = params,
    .num_parameters = 2,
};

void app_main() {
  // initialize naos
  naos_init(&config);
  naos_ble_init((naos_ble_config_t){});
  naos_wifi_init();
  naos_mqtt_init(1);
  naos_manager_init();

  // print foo
  naos_log("%d", foo_end - foo_start);
}
