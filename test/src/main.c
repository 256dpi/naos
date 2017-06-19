#include <naos.h>

static naos_config_t config = {.device_type = "naos-test",
                               .firmware_version = "0.0.1",
                               .loop_interval = 1000};

void app_main() {
  // initialize naos
  naos_init(&config);
}
