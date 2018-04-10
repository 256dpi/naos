#include <nvs.h>
#include <stdlib.h>
#include <string.h>

#include "utils.h"

static nvs_handle naos_manager_nvs_handle;

// TODO: Rename nvs namespace.

void naos_params_init() {
  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-manager", NVS_READWRITE, &naos_manager_nvs_handle));
}

char *naos_get(const char *param) {
  // static reference to buffer
  static char *buf;

  // free last param
  if (buf != NULL) {
    free(buf);
    buf = NULL;
  }

  // get param size
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_manager_nvs_handle, param, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    buf = strdup("");
    return buf;
  } else {
    ESP_ERROR_CHECK(err);
  }

  // allocate size
  buf = malloc(required_size);
  ESP_ERROR_CHECK(nvs_get_str(naos_manager_nvs_handle, param, buf, &required_size));

  return buf;
}

bool naos_get_b(const char *param) { return strtol(naos_get(param), NULL, 10) == 1; }

int32_t naos_get_l(const char *param) { return (int32_t)strtol(naos_get(param), NULL, 10); }

double naos_get_d(const char *param) { return strtod(naos_get(param), NULL); }

void naos_set(const char *param, const char *value) {
  // set parameter
  ESP_ERROR_CHECK(nvs_set_str(naos_manager_nvs_handle, param, value));
}

void naos_set_b(const char *param, bool value) { naos_set(param, naos_i2str(value)); }

void naos_set_l(const char *param, int32_t value) { naos_set(param, naos_i2str(value)); }

void naos_set_d(const char *param, double value) { naos_set(param, naos_d2str(value)); }

bool naos_ensure(const char *param, const char *value) {
  // check parameter
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_manager_nvs_handle, param, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    naos_set(param, value);
    return true;
  } else {
    ESP_ERROR_CHECK(err);
  }

  return false;
}

bool naos_ensure_b(const char *param, bool value) { return naos_ensure(param, naos_i2str(value)); }

bool naos_ensure_l(const char *param, int32_t value) { return naos_ensure(param, naos_i2str(value)); }

bool naos_ensure_d(const char *param, double value) { return naos_ensure(param, naos_d2str(value)); }

bool naos_unset(const char *param) {
  // erase parameter
  esp_err_t err = nvs_erase_key(naos_manager_nvs_handle, param);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    return false;
  } else {
    ESP_ERROR_CHECK(err);
  }

  return true;
}
