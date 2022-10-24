#include <esp_log.h>
#include <nvs.h>
#include <stdlib.h>
#include <string.h>

#include "naos.h"
#include "utils.h"

static nvs_handle naos_params_nvs_handle;
static naos_param_t *naos_params_registry[CONFIG_NAOS_PARAM_REGISTRY_SIZE] = {0};
static size_t naos_params_count = 0;

static void naos_params_update(naos_param_t *param) {
  // update pointer
  switch (param->type) {
    case NAOS_STRING: {
      // get value
      const char *value = naos_get(param->name);

      // update pointer
      if (param->sync_s != NULL) {
        if (*param->sync_s != NULL) {
          free(*param->sync_s);
        }
        *param->sync_s = strdup(value);
      }

      // yield value
      if (param->func_s != NULL) {
        param->func_s(value);
      }

      break;
    }
    case NAOS_BOOL: {
      // get value
      bool value = naos_get_b(param->name);

      // update pointer
      if (param->sync_b != NULL) {
        *param->sync_b = value;
      }

      // yield value
      if (param->func_b != NULL) {
        param->func_b(value);
      }

      break;
    }
    case NAOS_LONG: {
      // get value
      int32_t value = naos_get_l(param->name);

      // update pointer
      if (param->sync_l != NULL) {
        *param->sync_l = value;
      }

      // yield value
      if (param->func_l != NULL) {
        param->func_l(value);
      }

      break;
    }
    case NAOS_DOUBLE: {
      // get value
      double value = naos_get_d(param->name);

      // update pointer
      if (param->sync_d != NULL) {
        *param->sync_d = value;
      }

      // yield value
      if (param->func_d != NULL) {
        param->func_d(value);
      }

      break;
    }
  }
}

void naos_params_init() {
  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-app", NVS_READWRITE, &naos_params_nvs_handle));

  // register config parameters
  for (int i = 0; i < naos_config()->num_parameters; i++) {
    naos_register(&naos_config()->parameters[i]);
  }
}

void naos_register(naos_param_t *param) {
  // check size
  if (naos_params_count >= CONFIG_NAOS_PARAM_REGISTRY_SIZE) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store parameter
  naos_params_registry[naos_params_count] = param;
  naos_params_count++;

  // ensure parameter
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_params_nvs_handle, param->name, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    switch (param->type) {
      case NAOS_STRING:
        naos_set(param->name, param->default_s != NULL ? param->default_s : "");
        break;
      case NAOS_BOOL:
        naos_set_b(param->name, param->default_b);
        break;
      case NAOS_LONG:
        naos_set_l(param->name, param->default_l);
        break;
      case NAOS_DOUBLE:
        naos_set_d(param->name, param->default_d);
        break;
    }
  } else {
    ESP_ERROR_CHECK(err);
  }

  // update parameter
  naos_params_update(param);
}

naos_param_t *naos_lookup(const char *name) {
  // find parameter
  for (size_t i = 0; i < naos_params_count; i++) {
    naos_param_t *param = naos_params_registry[i];
    if (strcmp(name, param->name) == 0) {
      return param;
    }
  }

  return NULL;
}

char *naos_params_list() {
  // return empty string if there are no params
  if (naos_params_count == 0) {
    return strdup("");
  }

  // determine list length
  size_t length = 0;
  for (int i = 0; i < naos_params_count; i++) {
    length += strlen(naos_params_registry[i]->name) + 3;
  }

  // allocate buffer
  char *buf = malloc(length);

  // write names
  size_t pos = 0;
  for (int i = 0; i < naos_params_count; i++) {
    // get param
    naos_param_t *param = naos_params_registry[i];

    // copy name
    strcpy(buf + pos, param->name);
    pos += strlen(param->name);

    // write separator
    buf[pos] = ':';
    pos++;

    // write type
    switch (param->type) {
      case NAOS_STRING:
        buf[pos] = 's';
        break;
      case NAOS_BOOL:
        buf[pos] = 'b';
        break;
      case NAOS_LONG:
        buf[pos] = 'l';
        break;
      case NAOS_DOUBLE:
        buf[pos] = 'd';
        break;
    }
    pos++;

    // write comma or zero
    buf[pos] = (char)((i == naos_params_count - 1) ? '\0' : ',');
    pos++;
  }

  return buf;
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
  esp_err_t err = nvs_get_str(naos_params_nvs_handle, param, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    buf = strdup("");
    return buf;
  } else {
    ESP_ERROR_CHECK(err);
  }

  // allocate size
  buf = malloc(required_size);
  ESP_ERROR_CHECK(nvs_get_str(naos_params_nvs_handle, param, buf, &required_size));

  return buf;
}

bool naos_get_b(const char *param) { return strtol(naos_get(param), NULL, 10) == 1; }

int32_t naos_get_l(const char *param) { return (int32_t)strtol(naos_get(param), NULL, 10); }

double naos_get_d(const char *param) { return strtod(naos_get(param), NULL); }

void naos_set(const char *name, const char *value) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // set parameter
  ESP_ERROR_CHECK(nvs_set_str(naos_params_nvs_handle, name, value));

  // sync param
  naos_params_update(param);
}

void naos_set_b(const char *param, bool value) { naos_set(param, naos_i2str(value)); }

void naos_set_l(const char *param, int32_t value) { naos_set(param, naos_i2str(value)); }

void naos_set_d(const char *param, double value) { naos_set(param, naos_d2str(value)); }

bool naos_unset(const char *name) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // unset parameter
  esp_err_t err = nvs_erase_key(naos_params_nvs_handle, name);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    return false;
  } else {
    ESP_ERROR_CHECK(err);
  }

  // sync param
  naos_params_update(param);

  return true;
}
