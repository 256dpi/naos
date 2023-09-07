#include <naos/msg.h>
#include <naos/sys.h>

#include <stdlib.h>
#include <string.h>
#include <nvs_flash.h>

#include "naos.h"
#include "params.h"
#include "utils.h"

#define NAOS_PARAMS_ENDPOINT 0x01
#define NAOS_PARAMS_MAX_HANDLERS 8

typedef enum {
  NAOS_PARAMS_CMD_GET,
  NAOS_PARAMS_CMD_SET,
  NAOS_PARAMS_CMD_LIST,
  NAOS_PARAMS_CMD_READ,
  NAOS_PARAMS_CMD_WRITE,
  NAOS_PARAMS_CMD_COLLECT,
} naos_params_cmd_t;

static nvs_handle naos_params_handle;
static naos_mutex_t naos_params_mutex;
static naos_param_t *naos_params[CONFIG_NAOS_PARAM_REGISTRY_SIZE] = {0};
static size_t naos_params_count = 0;
static naos_params_handler_t naos_params_handlers[NAOS_PARAMS_MAX_HANDLERS] = {0};
static uint8_t naos_params_handler_count = 0;

static naos_value_t naos_params_default(naos_param_t *param) {
  // prepare scratch
  char scratch[32];

  // determine default value
  uint8_t *buf = NULL;
  size_t len = 0;
  switch (param->type) {
    case NAOS_RAW:
      buf = param->default_r.buf;
      len = param->default_r.len;
      break;
    case NAOS_STRING:
      if (param->default_s != 0) {
        buf = (uint8_t *)param->default_s;
        len = strlen(param->default_s);
      }
      break;
    case NAOS_BOOL:
      buf = (uint8_t *)naos_i2str(scratch, param->default_b);
      len = strlen((const char *)buf);
      break;
    case NAOS_LONG:
      buf = (uint8_t *)naos_i2str(scratch, param->default_l);
      len = strlen((const char *)buf);
      break;
    case NAOS_DOUBLE:
      buf = (uint8_t *)naos_d2str(scratch, param->default_d);
      len = strlen((const char *)buf);
      break;
    default:
      ESP_ERROR_CHECK(ESP_FAIL);
  }

  // copy value
  uint8_t *copy = malloc(len + 1);
  memcpy(copy, buf, len);
  copy[len] = 0;

  // return value
  return (naos_value_t){
      .buf = copy,
      .len = len,
  };
}

static void naos_params_update(naos_param_t *param, bool init) {
  // determine yield
  bool yield = !init || !param->skip_func_init;

  // handle type
  switch (param->type) {
    case NAOS_RAW: {
      // update pointer
      if (param->sync_r != NULL) {
        *param->sync_r = param->current;
      }

      // yield value
      if (yield && param->func_r != NULL) {
        param->func_r(param->current);
      }

      break;
    }
    case NAOS_STRING: {
      // update pointer
      if (param->sync_s != NULL) {
        if (*param->sync_s != NULL) {
          free(*param->sync_s);
        }
        if (param->current.buf != NULL) {
          *param->sync_s = strdup((const char *)param->current.buf);
        } else {
          *param->sync_s = strdup("");
        }
      }

      // yield value
      if (yield && param->func_s != NULL) {
        param->func_s((const char *)param->current.buf);
      }

      break;
    }
    case NAOS_BOOL: {
      // get value
      bool value = strtol((const char *)param->current.buf, NULL, 10) == 1;

      // update pointer
      if (param->sync_b != NULL) {
        *param->sync_b = value;
      }

      // yield value
      if (yield && param->func_b != NULL) {
        param->func_b(value);
      }

      break;
    }
    case NAOS_LONG: {
      // get value
      int32_t value = (int32_t)strtol((const char *)param->current.buf, NULL, 10);

      // update pointer
      if (param->sync_l != NULL) {
        *param->sync_l = value;
      }

      // yield value
      if (yield && param->func_l != NULL) {
        param->func_l(value);
      }

      break;
    }
    case NAOS_DOUBLE: {
      // get value
      double value = strtod((const char *)param->current.buf, NULL);

      // update pointer
      if (param->sync_d != NULL) {
        *param->sync_d = value;
      }

      // yield value
      if (yield && param->func_d != NULL) {
        param->func_d(value);
      }

      break;
    }
    case NAOS_ACTION: {
      // defer trigger
      if (yield && param->func_a != NULL) {
        naos_defer(param->func_a);
      }
    }
  }
}

static naos_msg_reply_t naos_params_process(naos_msg_t msg) {
  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // get command
  naos_params_cmd_t cmd = (naos_params_cmd_t)msg.data[0];

  // adjust message
  msg.data++;
  msg.len--;

  // handle command
  switch (cmd) {
    case NAOS_PARAMS_CMD_GET: {
      // command structure:
      // NAME (*)

      // check length
      if (msg.len == 0) {
        return NAOS_MSG_INVALID;
      }

      // get parameter
      naos_param_t *param = naos_lookup((const char *)msg.data);
      if (param == NULL) {
        return NAOS_MSG_ERROR;
      }

      // check type
      if (param->type == NAOS_ACTION) {
        return NAOS_MSG_ERROR;
      }

      // get value
      naos_value_t value = naos_get(param->name);

      // send reply
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = NAOS_PARAMS_ENDPOINT,
          .data = value.buf,
          .len = value.len,
      });

      return NAOS_MSG_OK;
    }

    case NAOS_PARAMS_CMD_SET: {
      // command structure:
      // NAME (*) | 0 | VALUE (*)

      // check length
      if (msg.len < 3) {
        return NAOS_MSG_INVALID;
      }

      // get name
      const char *name = (const char *)msg.data;

      // verify name
      if (strlen(name) == 0 || strlen(name) + 2 > msg.len) {
        return NAOS_MSG_INVALID;
      }

      // get parameter
      naos_param_t *param = naos_lookup(name);
      if (param == NULL) {
        return NAOS_MSG_ERROR;
      }

      // check type
      if (param->type == NAOS_ACTION) {
        return NAOS_MSG_ERROR;
      }

      // check mode
      if (param->mode & NAOS_LOCKED) {
        return NAOS_MSG_ERROR;
      }

      // set value
      naos_set(param->name, msg.data + strlen(name) + 1, msg.len - strlen(name) - 1);

      return NAOS_MSG_ACK;
    }

    case NAOS_PARAMS_CMD_LIST: {
      // check length
      if (msg.len != 0) {
        return NAOS_MSG_INVALID;
      }

      // reply structure
      // REF (1) | TYPE (1) | MODE (1) | NAME (*)

      // iterate parameters
      uint8_t data[256] = {0};
      for (int i = 0; i < naos_params_count; i++) {
        // get param
        naos_param_t *param = naos_params[i];

        // prepare data
        data[0] = i;
        data[1] = (uint8_t)param->type;
        data[2] = (uint8_t)param->mode;
        strcpy((char *)data + 3, param->name);

        // prepare reply
        naos_msg_t reply = {
            .session = msg.session,
            .endpoint = NAOS_PARAMS_ENDPOINT,
            .data = data,
            .len = 3 + strlen(param->name),
        };

        // send reply
        naos_msg_send(reply);
      }

      return NAOS_MSG_ACK;
    }

    case NAOS_PARAMS_CMD_READ: {
      // command structure:
      // REF (1)

      // check length
      if (msg.len != 1) {
        return NAOS_MSG_INVALID;
      }

      // check ref
      if (msg.data[0] >= naos_params_count) {
        return NAOS_MSG_ERROR;
      }

      // get parameter
      naos_param_t *param = naos_params[msg.data[0]];

      // check type
      if (param->type == NAOS_ACTION) {
        return NAOS_MSG_ERROR;
      }

      // get value
      naos_value_t value = naos_get(param->name);

      // send reply
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = NAOS_PARAMS_ENDPOINT,
          .data = value.buf,
          .len = value.len,
      });

      return NAOS_MSG_OK;
    }

    case NAOS_PARAMS_CMD_WRITE: {
      // command structure:
      // REF (1) | VALUE (*)

      // check length
      if (msg.len == 0) {
        return NAOS_MSG_INVALID;
      }

      // verify ref
      if (msg.data[0] >= naos_params_count) {
        return NAOS_MSG_ERROR;
      }

      // get parameter
      naos_param_t *param = naos_params[msg.data[0]];

      // check type
      if (param->type == NAOS_ACTION) {
        return NAOS_MSG_ERROR;
      }

      // set value
      naos_set(param->name, msg.data + 1, msg.len - 1);

      return NAOS_MSG_ACK;
    }

    case NAOS_PARAMS_CMD_COLLECT: {
      // command structure:
      // MAP (8) | SINCE (8)

      // check length
      if (msg.len != 16) {
        return NAOS_MSG_INVALID;
      }

      // get map
      uint64_t map = 0;
      memcpy(&map, msg.data, sizeof(uint64_t));

      // get since
      uint64_t since = 0;
      memcpy(&since, msg.data + 8, sizeof(uint64_t));

      // yield requested parameter values
      for (int i = 0; i < naos_params_count; i++) {
        // get param
        naos_param_t *param = naos_params[i];

        // skip action, unchanged, or not requested
        if (param->type == NAOS_ACTION || param->age < since || !(map & (1 << i))) {
          continue;
        }

        // reply structure
        // REF (1) | AGE (8) | VALUE (*)

        // prepare data
        uint8_t *data = malloc(9 + param->current.len);
        data[0] = i;
        memcpy(data + 1, &param->age, sizeof(uint64_t));
        memcpy(data + 9, param->current.buf, param->current.len);

        // prepare reply
        naos_msg_t reply = {
            .session = msg.session,
            .endpoint = NAOS_PARAMS_ENDPOINT,
            .data = data,
            .len = 9 + param->current.len,
        };

        // send reply
        naos_msg_send(reply);

        // free data
        free(data);
      }

      return NAOS_MSG_ACK;
    }

    default:
      return NAOS_MSG_UNKNOWN;
  }
}

void naos_params_init() {
  // create mutex
  naos_params_mutex = naos_mutex();

  // initialize flash memory
  ESP_ERROR_CHECK(nvs_flash_init());

  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos", NVS_READWRITE, &naos_params_handle));

  // register endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_PARAMS_ENDPOINT,
      .name = "params",
      .handle = naos_params_process,
  });
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
  if (!(param->mode & (NAOS_SYSTEM | NAOS_APPLICATION))) {
    param->mode |= NAOS_APPLICATION;
  }

  // store parameter
  naos_params[naos_params_count] = param;
  naos_params_count++;

  // handle actions
  if (param->type == NAOS_ACTION) {
    param->current = (naos_value_t){
        .buf = (uint8_t *)strdup(""),
        .len = 0,
    };
    NAOS_UNLOCK(naos_params_mutex);
    return;
  }

  // check existence
  size_t length;
  esp_err_t err = nvs_get_blob(naos_params_handle, param->name, NULL, &length);
  if ((param->mode & NAOS_VOLATILE) || err == ESP_ERR_NVS_NOT_FOUND) {
    // set default value if missing or volatile
    param->current = naos_params_default(param);
  } else if (err == ESP_OK) {
    // otherwise, load stored value
    uint8_t *buf = malloc(length + 1);
    ESP_ERROR_CHECK(nvs_get_blob(naos_params_handle, param->name, buf, &length));
    buf[length] = 0;

    // set value
    param->current = (naos_value_t){
        .buf = buf,
        .len = length,
    };
  } else {
    ESP_ERROR_CHECK(err);
  }

  // update parameter
  naos_params_update(param, true);

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);
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

  // determine list length
  size_t length = 0;
  size_t count = 0;
  for (int i = 0; i < naos_params_count; i++) {
    naos_param_t *param = naos_params[i];
    if ((param->mode & mode) == mode) {
      count++;
      length += strlen(param->name) + 4;
      if (param->mode & NAOS_VOLATILE) {
        length++;
      }
      if (param->mode & NAOS_SYSTEM) {
        length++;
      }
      if (param->mode & NAOS_APPLICATION) {
        length++;
      }
      if (param->mode & NAOS_LOCKED) {
        length++;
      }
    }
  }

  // return empty string if there are no params
  if (count == 0) {
    NAOS_UNLOCK(naos_params_mutex);
    return strdup("");
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
      case NAOS_RAW:
        buf[pos] = 'r';
        break;
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
    if (param->mode & NAOS_VOLATILE) {
      buf[pos] = 'v';
      pos++;
    }
    if (param->mode & NAOS_SYSTEM) {
      buf[pos] = 's';
      pos++;
    }
    if (param->mode & NAOS_APPLICATION) {
      buf[pos] = 'a';
      pos++;
    }
    if (param->mode & NAOS_LOCKED) {
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
  if (naos_params_handler_count >= NAOS_PARAMS_MAX_HANDLERS) {
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

naos_value_t naos_get(const char *name) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  return param->current;
}

const char *naos_get_s(const char *name) {
  // get parameter
  return (const char *)naos_get(name).buf;
}

bool naos_get_b(const char *param) {
  // get parameter
  return strtol(naos_get_s(param), NULL, 10) == 1;
}

int32_t naos_get_l(const char *param) {
  // get parameter
  return (int32_t)strtol(naos_get_s(param), NULL, 10);
}

double naos_get_d(const char *param) {
  // get parameter
  return strtod(naos_get_s(param), NULL);
}

void naos_set(const char *name, uint8_t *value, size_t length) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // check value
  if (length > 0 && value == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // store value if not volatile
  if (!(param->mode & NAOS_VOLATILE)) {
    ESP_ERROR_CHECK(nvs_set_blob(naos_params_handle, param->name, value, length));
  }

  // free last value
  if (param->last.buf != NULL) {
    free(param->last.buf);
  }

  // move current to last value
  param->last = param->current;

  // copy value
  uint8_t *copy = malloc(length + 1);
  memcpy(copy, value, length);
  copy[length] = 0;

  // set current value
  param->current = (naos_value_t){
      .buf = copy,
      .len = length,
  };

  // track change
  param->changed = true;
  param->age = naos_millis();

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);

  // update parameter
  naos_params_update(param, false);
}

void naos_set_s(const char *param, const char *value) {
  // set parameter
  naos_set(param, (uint8_t *)value, value != NULL ? strlen(value) : 0);
}

void naos_set_b(const char *param, bool value) {
  // set parameter
  char buf[16] = {0};
  naos_set_s(param, naos_i2str(buf, value));
}

void naos_set_l(const char *param, int32_t value) {
  // set parameter
  char buf[16] = {0};
  naos_set_s(param, naos_i2str(buf, value));
}

void naos_set_d(const char *param, double value) {
  // set parameter
  char buf[32] = {0};
  naos_set_s(param, naos_d2str(buf, value));
}

void naos_clear(const char *name) {
  // lookup parameter
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // acquire mutex
  NAOS_LOCK(naos_params_mutex);

  // erase value if not volatile
  if (!(param->mode & NAOS_VOLATILE)) {
    ESP_ERROR_CHECK(nvs_erase_key(naos_params_handle, param->name));
  }

  // free last value
  if (param->last.buf != NULL) {
    free(param->last.buf);
  }

  // move current to last value
  param->last = param->current;

  // set current value
  param->current = naos_params_default(param);

  // track change
  param->changed = true;
  param->age = naos_millis();

  // release mutex
  NAOS_UNLOCK(naos_params_mutex);

  // update parameter
  naos_params_update(param, false);
}
