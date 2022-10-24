#include <esp_log.h>
#include <nvs.h>
#include <stdlib.h>
#include <string.h>

#include "naos.h"
#include "utils.h"

#define NAOS_PARAMS_SIZE 64

typedef struct {
  naos_type_t type;
  const char *param;
  void *pointer;
  void(*func);
} naos_params_sync_item_t;

static nvs_handle naos_params_nvs_handle;
static naos_param_t *naos_params[NAOS_PARAMS_SIZE] = {0};
static size_t naos_params_count = 0;
static naos_params_sync_item_t naos_params_sync_registry[CONFIG_NAOS_SYNC_REGISTRY_SIZE] = {0};
static size_t naos_params_sync_registry_count = 0;

static bool naos_params_add_sync(const char *param, naos_params_sync_item_t item) {
  // check param length
  if (strlen(param) == 0) {
    return false;
  }

  // check registry count
  if (naos_params_sync_registry_count >= CONFIG_NAOS_SYNC_REGISTRY_SIZE) {
    ESP_LOGE(NAOS_LOG_TAG, "naos_params_add_sync: registry full");
    return false;
  }

  // add entry to registry
  naos_params_sync_registry[naos_params_sync_registry_count] = item;

  // increment counter
  naos_params_sync_registry_count++;

  return true;
}

static void naos_params_update_sync(const char *param) {
  // update synchronized variables
  for (size_t i = 0; i < naos_params_sync_registry_count; i++) {
    // get item
    naos_params_sync_item_t item = naos_params_sync_registry[i];

    // check param
    if (strcmp(item.param, param) != 0) {
      continue;
    }

    // update pointer
    switch (item.type) {
      case NAOS_STRING: {
        // get value
        char *value = naos_get(param);

        // update pointer
        if (item.pointer != NULL) {
          char **pointer = (char **)item.pointer;
          if (*pointer != NULL) {
            free(*pointer);
          }
          *pointer = strdup(value);
        }

        // yield value
        if (item.func != NULL) {
          void (*func)(char *) = item.func;
          func(value);
        }

        break;
      }
      case NAOS_BOOL: {
        // get value
        bool value = naos_get_b(param);

        // update pointer
        if (item.pointer != NULL) {
          bool *pointer = (bool *)item.pointer;
          *pointer = value;
        }

        // yield value
        if (item.func != NULL) {
          void (*func)(bool) = item.func;
          func(value);
        }

        break;
      }
      case NAOS_LONG: {
        // get value
        int32_t value = naos_get_l(param);

        // update pointer
        if (item.pointer != NULL) {
          int32_t *pointer = (int32_t *)item.pointer;
          *pointer = value;
        }

        // yield value
        if (item.func != NULL) {
          void (*func)(int32_t) = item.func;
          func(value);
        }

        break;
      }
      case NAOS_DOUBLE: {
        // get value
        double value = naos_get_d(param);

        // update pointer
        if (item.pointer != NULL) {
          double *pointer = (double *)item.pointer;
          *pointer = value;
        }

        // yield value
        if (item.func != NULL) {
          void (*func)(double) = item.func;
          func(value);
        }

        break;
      }
    }
  }
}

static bool naos_ensure(const char *param, const char *value) {
  // check parameter
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_params_nvs_handle, param, NULL, &required_size);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    naos_set(param, value);
    return true;
  } else {
    ESP_ERROR_CHECK(err);
  }

  return false;
}

static bool naos_ensure_b(const char *param, bool value) { return naos_ensure(param, naos_i2str(value)); }

static bool naos_ensure_l(const char *param, int32_t value) { return naos_ensure(param, naos_i2str(value)); }

static bool naos_ensure_d(const char *param, double value) { return naos_ensure(param, naos_d2str(value)); }

static bool naos_sync(const char *param, char **pointer, void (*func)(char *)) {
  // prepare item
  naos_params_sync_item_t item = {
      .type = NAOS_STRING,
      .param = param,
      .pointer = (void *)pointer,
      .func = func,
  };

  // add sync item
  bool ret = naos_params_add_sync(param, item);

  // get value
  char *value = naos_get(param);

  // set value
  if (pointer != NULL) {
    *pointer = strdup(value);
  }

  // yield value
  if (func != NULL) {
    func(value);
  }

  return ret;
}

static bool naos_sync_b(const char *param, bool *pointer, void (*func)(bool)) {
  // prepare item
  naos_params_sync_item_t item = {
      .type = NAOS_BOOL,
      .param = param,
      .pointer = (void *)pointer,
      .func = func,
  };

  // add sync item
  bool ret = naos_params_add_sync(param, item);

  // get value
  bool value = naos_get_b(param);

  // set value
  if (pointer != NULL) {
    *pointer = value;
  }

  // yield value
  if (func != NULL) {
    func(value);
  }

  return ret;
}

static bool naos_sync_l(const char *param, int32_t *pointer, void (*func)(int32_t)) {
  // prepare item
  naos_params_sync_item_t item = {
      .type = NAOS_LONG,
      .param = param,
      .pointer = (void *)pointer,
      .func = func,
  };

  // add sync item
  bool ret = naos_params_add_sync(param, item);

  // get value
  int32_t value = naos_get_l(param);

  // set value
  if (pointer != NULL) {
    *pointer = value;
  }

  // yield value
  if (func != NULL) {
    func(value);
  }

  return ret;
}

static bool naos_sync_d(const char *param, double *pointer, void (*func)(double)) {
  // prepare item
  naos_params_sync_item_t item = {
      .type = NAOS_DOUBLE,
      .param = param,
      .pointer = (void *)pointer,
      .func = func,
  };

  // add sync item
  bool ret = naos_params_add_sync(param, item);

  // get value
  double value = naos_get_d(param);

  // set value
  if (pointer != NULL) {
    *pointer = value;
  }

  // yield value
  if (func != NULL) {
    func(value);
  }

  return ret;
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
  if (naos_params_count >= NAOS_PARAMS_SIZE) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store parameter
  naos_params[naos_params_count] = param;
  naos_params_count++;

  // ensure parameter
  switch (param->type) {
    case NAOS_STRING:
      naos_ensure(param->name, param->default_s != NULL ? param->default_s : "");
      break;
    case NAOS_BOOL:
      naos_ensure_b(param->name, param->default_b);
      break;
    case NAOS_LONG:
      naos_ensure_l(param->name, param->default_l);
      break;
    case NAOS_DOUBLE:
      naos_ensure_d(param->name, param->default_d);
      break;
  }

  // setup synchronization
  switch (param->type) {
    case NAOS_STRING:
      if (param->sync_s != NULL || param->func_s != NULL) {
        naos_sync(param->name, param->sync_s, param->func_s);
      }
      break;
    case NAOS_BOOL:
      if (param->sync_b != NULL || param->func_b != NULL) {
        naos_sync_b(param->name, param->sync_b, param->func_b);
      }
      break;
    case NAOS_LONG:
      if (param->sync_l != NULL || param->func_l != NULL) {
        naos_sync_l(param->name, param->sync_l, param->func_l);
      }
      break;
    case NAOS_DOUBLE:
      if (param->sync_d != NULL || param->func_d != NULL) {
        naos_sync_d(param->name, param->sync_d, param->func_d);
      }
      break;
  }
}

naos_param_t *naos_lookup(const char *name) {
  // find parameter
  for (size_t i = 0; i < naos_params_count; i++) {
    naos_param_t *param = naos_params[i];
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
    length += strlen(naos_params[i]->name) + 3;
  }

  // allocate buffer
  char *buf = malloc(length);

  // write names
  size_t pos = 0;
  for (int i = 0; i < naos_params_count; i++) {
    // get param
    naos_param_t *param = naos_params[i];

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

void naos_set(const char *param, const char *value) {
  // set parameter
  ESP_ERROR_CHECK(nvs_set_str(naos_params_nvs_handle, param, value));

  // sync param
  naos_params_update_sync(param);
}

void naos_set_b(const char *param, bool value) { naos_set(param, naos_i2str(value)); }

void naos_set_l(const char *param, int32_t value) { naos_set(param, naos_i2str(value)); }

void naos_set_d(const char *param, double value) { naos_set(param, naos_d2str(value)); }

bool naos_unset(const char *param) {
  // erase parameter
  esp_err_t err = nvs_erase_key(naos_params_nvs_handle, param);
  if (err == ESP_ERR_NVS_NOT_FOUND) {
    return false;
  } else {
    ESP_ERROR_CHECK(err);
  }

  // sync param
  naos_params_update_sync(param);

  return true;
}
