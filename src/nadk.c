#include <nadk.h>

#include "system.h"

static nadk_config_t *nadk_config_ref;

void nadk_init(nadk_config_t *config) {
  // set config reference
  nadk_config_ref = config;

  // initialize system
  nadk_system_init();
}

const nadk_config_t *nadk_config() { return nadk_config_ref; }

const char *nadk_scope_str(nadk_scope_t scope) {
  switch (scope) {
    case NADK_LOCAL:
      return "local";
    case NADK_GLOBAL:
      return "global";
  }

  return "";
}

const char *nadk_status_str(nadk_status_t status) {
  switch (status) {
    case NADK_DISCONNECTED:
      return "disconnected";
    case NADK_CONNECTED:
      return "connected";
    case NADK_NETWORKED:
      return "networked";
  }

  return "";
}
