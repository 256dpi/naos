#include <naos.h>

#include "system.h"

static naos_config_t *naos_config_ref;

void naos_init(naos_config_t *config) {
  // set config reference
  naos_config_ref = config;

  // initialize system
  naos_system_init();
}

const naos_config_t *naos_config() {
  // return config
  return naos_config_ref;
}

const char *naos_scope_str(naos_scope_t scope) {
  switch (scope) {
    case NAOS_LOCAL:
      return "local";
    case NAOS_GLOBAL:
      return "global";
    default:
      return "";
  }
}

const char *naos_status_str(naos_status_t status) {
  switch (status) {
    case NAOS_DISCONNECTED:
      return "disconnected";
    case NAOS_CONNECTED:
      return "connected";
    case NAOS_NETWORKED:
      return "networked";
    default:
      return "";
  }
}
