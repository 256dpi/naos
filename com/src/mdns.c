#include <naos.h>
#include <naos/sys.h>
#include <naos/mdns.h>
#include <mdns.h>
#include <string.h>

#include "params.h"

static naos_mdns_config_t naos_mdns_config = {0};

static void naos_mdns_params_handler(naos_param_t *param) {
  // update hostname and instance name
  if (strcmp(param->name, "device-name") == 0) {
    const char *name = naos_get_s("device-name");
    if (strlen(name) > 0) {
      ESP_ERROR_CHECK(mdns_hostname_set(name));
      ESP_ERROR_CHECK(mdns_instance_name_set(name));
    } else {
      ESP_ERROR_CHECK(mdns_hostname_set("naos"));
      ESP_ERROR_CHECK(mdns_instance_name_set("naos"));
    }
  }

  // update OSC port
  if (strcmp(param->name, "osc-port") == 0 && naos_mdns_config.osc) {
    int port = naos_get_l("osc-port");
    esp_err_t err = mdns_service_remove("_naos_osc", "_udp");
    if (err != ESP_ERR_NOT_FOUND) {
      ESP_ERROR_CHECK(err);
    }
    if (port > 0) {
      ESP_ERROR_CHECK(mdns_service_add(NULL, "_naos_osc", "_udp", port, NULL, 0));
    }
  }
}

void naos_mdns_announce() {
  // trigger announcement
  ESP_ERROR_CHECK(mdns_service_port_set("_naos", "_tcp", 1));
}

void naos_mdns_init(naos_mdns_config_t config) {
  // store config
  naos_mdns_config = config;

  // initialize mDNS service
  ESP_ERROR_CHECK(mdns_init());

  // set hostname and instance name
  const char *name = naos_get_s("device-name");
  if (strlen(name) > 0) {
    ESP_ERROR_CHECK(mdns_hostname_set(naos_get_s("device-name")));
    ESP_ERROR_CHECK(mdns_instance_name_set(naos_get_s("device-name")));
  } else {
    ESP_ERROR_CHECK(mdns_hostname_set("naos"));
    ESP_ERROR_CHECK(mdns_instance_name_set("naos"));
  }

  // register main service if available
  if (config.main) {
    ESP_ERROR_CHECK(mdns_service_add(NULL, "_naos", "_tcp", 1, NULL, 0));
  }

  // register HTTP service if available
  if (config.http) {
    ESP_ERROR_CHECK(mdns_service_add(NULL, "_naos_http", "_tcp", 80, NULL, 0));
  }

  // register OSC service if available
  if (config.osc) {
    int port = naos_get_l("osc-port");
    if (port > 0) {
      ESP_ERROR_CHECK(mdns_service_add(NULL, "_naos_osc", "_udp", port, NULL, 0));
    }
  }

  // register params handler
  naos_params_subscribe(naos_mdns_params_handler);

  // periodically announce main service
  if (config.main) {
    naos_repeat("mdns", 1000, naos_mdns_announce);
  }
}
