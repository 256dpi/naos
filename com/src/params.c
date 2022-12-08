#include <naos_sys.h>

#include <stdlib.h>
#include <string.h>
#include <esp_log.h>
#include <nvs_flash.h>

#include "naos.h"
#include "params.h"
#include "utils.h"

#define NAOS_PARAMS_MAX_RECEIVERS 8

static nvs_handle naos_params_handle;
static naos_mutex_t naos_params_mutex;
static naos_param_t *naos_params[CONFIG_NAOS_PARAM_REGISTRY_SIZE] = {0};
static size_t naos_params_count = 0;
static naos_params_handler_t naos_params_handlers[NAOS_PARAMS_MAX_RECEIVERS] = {0};
static uint8_t naos_params_handler_count = 0;

static void naos_params_update(naos_param_t *param) {
  // update pointer
  switch (param->type) {
    case NAOS_STRING: {
      // update pointer
      if (param->sync_s != NULL) {
        if (*param->sync_s != NULL) {
          free(*param->sync_s);
        }
        *param->sync_s = strdup(param->value);
      }

      // yield value
      if (param->func_s != NULL) {
        param->func_s(param->value);
      }

      break;
    }
    case NAOS_BOOL: {
      // get value
      bool value = strtol(param->value, NULL, 10) == 1;

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
      int32_t value = (int32_t)strtol(param->value, NULL, 10);

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
      double value = strtod(param->value, NULL);

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
    case NAOS_ACTION: {
      // defer trigger
      if (param->func_a != NULL) {
        naos_defer(param->func_a);
      }
    }
  }
}

void naos_params_init() {
  // create mutex
  naos_params_mutex = naos_mutex();

  // initialize flash memory
  ESP_ERROR_CHECK(nvs_flash_init());

  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos", NVS_READWRITE, &naos_params_handle));
}

void naos_register(naos_param_t *param) {
  // check name and type
  if (strlen(param->name) == 0) {
    ESP_ERROR_CHECK(ESP_FAIL);
  } else if (naos_lookup(param->name) != NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  } else if (param->type < 0 || param->type > NAOS_ACTION) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // check size
  if (naos_params_count >= CONFIG_NAOS_PARAM_REGISTRY_SIZE) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // force volatile for actions
  if (param->type == NAOS_ACTION) {
    param->mode |= NAOS_VOLATILE;
  }

  // force application if undefined
  if ((param->mode & (NAOS_SYSTEM | NAOS_APPLICATION)) == 0) {
    param->mode |= NAOS_APPLICATION;
  }

  // store parameter
  naos_params[naos_params_count] = param;
  naos_params_count++;

  // check parameter
  size_t required_size;
  esp_err_t err = nvs_get_str(naos_params_handle, param->name, NULL, &required_size);
  if ((param->mode & NAOS_VOLATILE) != 0 || err == ESP_ERR_NVS_NOT_FOUND) {
    // set default value if missing or volatile
    NAOS_UNLOCK(naos_params_mutex);
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
      case NAOS_ACTION:
        param->value = strdup("");
        break;
    }
    NAOS_LOCK(naos_params_mutex);
  } else if (err == ESP_OK) {
    // otherwise, read stored value
    char *buf = malloc(required_size);
    ESP_ERROR_CHECK(nvs_get_str(naos_params_handle, param->name, buf, &required_size));
    param->value = buf;
  } else {
    ESP_ERROR_CHECK(err);
  }

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);

  // update parameter if not action
  if (param->type != NAOS_ACTION) {
    naos_params_update(param);
  }
}

naos_param_t *naos_lookup(const char *name) {
  // check name
  if (name == NULL || strlen(name) == 0) {
    return NULL;
  }

  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // find parameter
  naos_param_t *result = NULL;
  for (size_t i = 0; i < naos_params_count; i++) {
    naos_param_t *param = naos_params[i];
    if (strcmp(name, param->name) == 0) {
      result = param;
      break;
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);

  return result;
}

char *naos_params_list(naos_mode_t mode) {
  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // return empty string if there are no params
  if (naos_params_count == 0) {
    NAOS_UNLOCK(naos_params_mutex);
    return strdup("");
  }

  // determine list length
  size_t length = 0;
  size_t count = 0;
  for (int i = 0; i < naos_params_count; i++) {
    naos_param_t *param = naos_params[i];
    if ((param->mode & mode) == mode) {
      count++;
      length += strlen(param->name) + 4;
      if ((param->mode & NAOS_VOLATILE) != 0) {
        length++;
      }
      if ((param->mode & NAOS_SYSTEM) != 0) {
        length++;
      }
      if ((param->mode & NAOS_APPLICATION) != 0) {
        length++;
      }
      if ((param->mode & NAOS_PUBLIC) != 0) {
        length++;
      }
      if ((param->mode & NAOS_LOCKED) != 0) {
        length++;
      }
    }
  }

  // allocate buffer
  char *buf = malloc(length);

  // write names
  size_t pos = 0;
  for (int i = 0; i < naos_params_count; i++) {
    // get param
    naos_param_t *param = naos_params[i];

    // check mode
    if ((param->mode & mode) != mode) {
      continue;
    }

    // decrement
    count--;

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
      case NAOS_ACTION:
        buf[pos] = 'a';
        break;
      default:
        buf[pos] = '?';
        break;
    }
    pos++;

    // write separator
    buf[pos] = ':';
    pos++;

    // write mode
    if ((param->mode & NAOS_VOLATILE) != 0) {
      buf[pos] = 'v';
      pos++;
    }
    if ((param->mode & NAOS_SYSTEM) != 0) {
      buf[pos] = 's';
      pos++;
    }
    if ((param->mode & NAOS_APPLICATION) != 0) {
      buf[pos] = 'a';
      pos++;
    }
    if ((param->mode & NAOS_PUBLIC) != 0) {
      buf[pos] = 'p';
      pos++;
    }
    if ((param->mode & NAOS_LOCKED) != 0) {
      buf[pos] = 'l';
      pos++;
    }

    // write comma or zero
    buf[pos] = (char)((count == 0) ? '\0' : ',');
    pos++;
  }

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);

  return buf;
}

void naos_params_subscribe(naos_params_handler_t handler) {
  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // check count
  if (naos_params_handler_count >= NAOS_PARAMS_MAX_RECEIVERS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // add handler
  naos_params_handlers[naos_params_handler_count] = handler;
  naos_params_handler_count++;

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);
}

void naos_params_dispatch() {
  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // iterate parameters
  for (size_t i = 0; i < naos_params_count; i++) {
    // get param
    naos_param_t *param = naos_params[i];

    // continue if not changed
    if (!param->changed) {
      continue;
    }

    // clear flag
    param->changed = false;

    // dispatch change
    for (size_t j = 0; j < naos_params_handler_count; j++) {
      NAOS_UNLOCK(naos_params_mutex);
      naos_params_handlers[j](param);
      NAOS_LOCK(naos_params_mutex);
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);
}

const char *naos_get(const char *name) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  return param->value;
}

bool naos_get_b(const char *param) {
  // get parameter
  return strtol(naos_get(param), NULL, 10) == 1;
}

int32_t naos_get_l(const char *param) {
  // get parameter
  return (int32_t)strtol(naos_get(param), NULL, 10);
}

double naos_get_d(const char *param) {
  // get parameter
  return strtod(naos_get(param), NULL);
}

void naos_set(const char *name, const char *value) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // check value
  if (value == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // set parameter if not volatile
  if ((param->mode & NAOS_VOLATILE) == 0) {
    ESP_ERROR_CHECK(nvs_set_str(naos_params_handle, name, value));
  }

  // update value
  free(param->value);
  param->value = strdup(value);

  // track change
  param->changed = true;

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);

  // update parameter
  naos_params_update(param);
}

void naos_set_b(const char *param, bool value) {
  // set parameter
  char buf[16] = {0};
  naos_set(param, naos_i2str(buf, value));
}

void naos_set_l(const char *param, int32_t value) {
  // set parameter
  char buf[16] = {0};
  naos_set(param, naos_i2str(buf, value));
}

void naos_set_d(const char *param, double value) {
  // set parameter
  char buf[16] = {0};
  naos_set(param, naos_d2str(buf, value));
}
