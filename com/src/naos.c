#include <naos.h>
#include <esp_app_desc.h>

#include "system.h"

static naos_config_t *naos_config_ref;

void naos_init(naos_config_t *config) {
  // set config reference
  naos_config_ref = config;

  // ensure app name and version
  const esp_app_desc_t *desc = esp_app_get_description();
  if (config->app_name == NULL) {
    config->app_name = desc->project_name;
  }
  if (config->app_version == NULL) {
    config->app_version = desc->version;
  }

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
