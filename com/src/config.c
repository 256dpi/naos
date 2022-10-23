#include <stdlib.h>
#include <string.h>
#include <esp_err.h>
#include <esp_system.h>

#include "config.h"
#include "settings.h"
#include "params.h"
#include "manager.h"
#include "system.h"
#include "utils.h"
#include "naos.h"

#define NAOS_CONFIG_MAX_HANDLERS 8
static naos_config_handler_t naos_config_handlers[NAOS_CONFIG_MAX_HANDLERS] = {0};
static uint8_t naos_config_num_handlers = 0;

char* naos_config_identify() {
  // get type and name
  const char* type = naos_config()->device_type;
  char* name = naos_settings_read(NAOS_SETTING_DEVICE_NAME);

  // assemble string
  char* str = naos_format("device_type=%s,device_name=%s", type, name);
  free(name);

  return str;
}

char* naos_config_describe() {
  // get status
  const char* status = naos_status_str(naos_status());

  // get battery
  double battery = -1;
  if (naos_config()->battery_level != NULL) {
    battery = naos_config()->battery_level();
  }

  // assemble string
  char* str = naos_format("connection_status=%s,battery_level=%.2f", status, battery);

  return str;
}

char* naos_config_list_settings() {
  // list settings
  return naos_settings_list();
}

char* naos_config_read_setting(const char* key) {
  // read setting
  return naos_settings_read(naos_setting_from_key(key));
}

void naos_config_write_setting(const char* key, const char* value) {
  // write setting
  naos_settings_write(naos_setting_from_key(key), value);
}

void naos_config_execute(const char* command) {
  // handle command
  if (strcmp(command, "ping") == 0) {
    if (naos_config()->ping_callback != NULL) {
      naos_acquire();
      naos_config()->ping_callback();
      naos_release();
    }
  } else if (strcmp(command, "reboot") == 0) {
    esp_restart();
  } else if (strcmp(command, "restart-wifi") == 0) {
    naos_system_configure_wifi();
  } else if (strcmp(command, "restart-mqtt") == 0) {
    naos_system_configure_mqtt();
  }
}

char* naos_config_list_params() {
  // list params
  return naos_params_list();
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
