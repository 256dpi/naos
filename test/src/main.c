#include <naos.h>

static naos_config_t config = {.device_type = "my-device",
                               .firmware_version = "0.0.1"};

void app_main() {
  // initialize naos
  naos_init(&config);
}
