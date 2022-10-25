#include <esp_err.h>

#include "config.h"
#include "params.h"
#include "manager.h"

#define NAOS_CONFIG_MAX_HANDLERS 8
static naos_config_handler_t naos_config_handlers[NAOS_CONFIG_MAX_HANDLERS] = {0};
static uint8_t naos_config_num_handlers = 0;

char* naos_config_list_params(naos_mode_t mode) {
  // list params
  return naos_params_list(mode);
}

char* naos_config_read_param(const char* key) {
  // read param
  return naos_manager_read_param(naos_lookup(key));
}

void naos_config_write_parm(const char* key, const char* value) {
  // write param
  return naos_manager_write_param(naos_lookup(key), value);
}

void naos_config_register(naos_config_handler_t handler) {
  // check size
  if (naos_config_num_handlers >= NAOS_CONFIG_MAX_HANDLERS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // add handler
  naos_config_handlers[naos_config_num_handlers] = handler;
  naos_config_num_handlers++;
}

void naos_config_notify(naos_config_notification_t notification) {
  // run handlers
  for (uint8_t i = 0; i < naos_config_num_handlers; i++) {
    naos_config_handlers[i](notification);
  }
}
