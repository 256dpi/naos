#include <naos_sys.h>

#include <esp_bt.h>
#include <esp_bt_defs.h>
#include <esp_bt_main.h>
#include <esp_gap_ble_api.h>
#include <esp_gatt_defs.h>
#include <esp_gatts_api.h>
#include <string.h>

#include "naos.h"
#include "params.h"
#include "utils.h"

#define NAOS_BLE_MAX_CONNECTIONS CONFIG_BT_ACL_CONNECTIONS

typedef struct {
  uint16_t id;
  bool connected;
  bool locked;
  naos_mode_t mode;
  naos_param_t *param;
} naos_ble_conn_t;

static naos_mutex_t naos_ble_mutex;
static naos_signal_t naos_ble_signal;

static esp_ble_adv_params_t naos_ble_adv_params = {
    .adv_int_min = 0x20,
    .adv_int_max = 0x40,
    .adv_type = ADV_TYPE_IND,
    .own_addr_type = BLE_ADDR_TYPE_PUBLIC,
    .channel_map = ADV_CHNL_ALL,
    .adv_filter_policy = ADV_FILTER_ALLOW_SCAN_ANY_CON_ANY,
};

static esp_ble_adv_data_t naos_ble_adv_data = {
    .set_scan_rsp = false,
    .include_name = true,
    .include_txpower = true,
    .min_interval = 0x20, // 40ms
    .max_interval = 0x40, // 80ms
    .flag = ESP_BLE_ADV_FLAG_GEN_DISC | ESP_BLE_ADV_FLAG_BREDR_NOT_SPT,
};

static struct {
  uint8_t uuid[16];
  // ---
  esp_gatt_if_t interface;
  esp_gatt_srvc_id_t service_id;
  uint16_t service_handle;
} naos_ble_gatts_profile = {
    .uuid = {0xB5, 0x33, 0x50, 0x9D, 0xEE, 0xFF, 0x03, 0x81, 0x4F, 0x4E, 0x61, 0x48, 0x1B, 0xBA, 0x2F, 0x63}};

typedef struct {
  uint8_t uuid[16];
  esp_gatt_char_prop_t prop;
  uint16_t max_write_len;
  // ---
  uint16_t handle;
  esp_bt_uuid_t _uuid;
} naos_ble_gatts_char_t;

static naos_ble_gatts_char_t naos_ble_char_lock = {
    .uuid = {0x91, 0xb5, 0x2e, 0x90, 0xd5, 0x07, 0x4d, 0x68, 0x9b, 0x23, 0x84, 0x40, 0xa4, 0xfb, 0xa5, 0xf7},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 32};

static naos_ble_gatts_char_t naos_ble_char_list = {
    .uuid = {0x65, 0xa6, 0x6e, 0x1a, 0x95, 0x7d, 0x48, 0xdf, 0x8b, 0xb7, 0x1b, 0x23, 0xd1, 0x89, 0x22, 0xac},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 32};

static naos_ble_gatts_char_t naos_ble_char_select = {
    .uuid = {0xcd, 0xba, 0xd4, 0x6e, 0x8d, 0xf8, 0x40, 0x42, 0xbe, 0xcc, 0x6f, 0x40, 0x6d, 0x70, 0xc9, 0xcf},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 64};

static naos_ble_gatts_char_t naos_ble_char_value = {
    .uuid = {0xb3, 0x71, 0x1e, 0xb0, 0x84, 0x68, 0x41, 0x20, 0x99, 0x7e, 0xe1, 0x8e, 0x46, 0x54, 0xca, 0x01},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 256};

static naos_ble_gatts_char_t naos_ble_char_update = {
    .uuid = {0x26, 0x17, 0x8c, 0xbc, 0x61, 0x7a, 0x4a, 0x9c, 0xa2, 0x22, 0x04, 0x07, 0xcf, 0xfd, 0xbf, 0x87},
    .prop = ESP_GATT_CHAR_PROP_BIT_INDICATE};

#define NAOS_BLE_NUM_CHARS 5

static naos_ble_gatts_char_t *naos_ble_gatts_chars[NAOS_BLE_NUM_CHARS] = {
    &naos_ble_char_lock, &naos_ble_char_list, &naos_ble_char_select, &naos_ble_char_value, &naos_ble_char_update,
};

static naos_ble_conn_t naos_ble_conns[NAOS_BLE_MAX_CONNECTIONS];

static void naos_ble_gap_handler(esp_gap_ble_cb_event_t e, esp_ble_gap_cb_param_t *p) {
  switch (e) {
    case ESP_GAP_BLE_ADV_DATA_SET_COMPLETE_EVT: {
      // begin with advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    case ESP_GAP_BLE_ADV_START_COMPLETE_EVT: {
      // check status
      if (p->adv_start_cmpl.status != ESP_BT_STATUS_SUCCESS) {
        ESP_LOGE(NAOS_LOG_TAG, "ESP_GAP_BLE_ADV_START_COMPLETE_EVT: %d", p->adv_data_raw_cmpl.status);
      }

      break;
    }

    default: {
      // ESP_LOGI(NAOS_LOG_TAG, "unhandled GAP event: %d", e);
    }
  }
}

static void naos_ble_gatts_handler(esp_gatts_cb_event_t e, esp_gatt_if_t i, esp_ble_gatts_cb_param_t *p) {
  // acquire mutex
  NAOS_LOCK(naos_ble_mutex);

  // pre-check for registration event
  if (e == ESP_GATTS_REG_EVT) {
    ESP_ERROR_CHECK(p->reg.status);

    // store gatts interface after registration
    naos_ble_gatts_profile.interface = i;
  }

  // return immediately if event is not general or does not belong to our interface
  if (i != ESP_GATT_IF_NONE && i != naos_ble_gatts_profile.interface) {
    NAOS_UNLOCK(naos_ble_mutex);
    return;
  }

  // triage event
  switch (e) {
    // handle registration event (status has been handled above)
    case ESP_GATTS_REG_EVT: {
      // set advertisement config
      ESP_ERROR_CHECK(esp_ble_gap_config_adv_data(&naos_ble_adv_data));

      // count required handles for service
      uint16_t total_handles = 1;
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];
        total_handles += 2;
        if (c->prop & ESP_GATT_CHAR_PROP_BIT_INDICATE) {
          total_handles++;
        }
      }

      // create service
      ESP_ERROR_CHECK(esp_ble_gatts_create_service(i, &naos_ble_gatts_profile.service_id, total_handles));

      break;
    }

    // handle service creation event
    case ESP_GATTS_CREATE_EVT: {
      // check status
      ESP_ERROR_CHECK(p->create.status);

      // save assigned handle
      naos_ble_gatts_profile.service_handle = p->create.service_handle;

      // start service
      ESP_ERROR_CHECK(esp_ble_gatts_start_service(naos_ble_gatts_profile.service_handle));

      // iterate through all characteristics
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        // get pointer of current characteristic
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

        // prepare control
        esp_attr_control_t control = {.auto_rsp = ESP_GATT_RSP_BY_APP};

        // prepare permissions
        esp_gatt_perm_t perm = ESP_GATT_PERM_READ | ESP_GATT_PERM_WRITE;

        // add characteristic
        ESP_ERROR_CHECK(
            esp_ble_gatts_add_char(naos_ble_gatts_profile.service_handle, &c->_uuid, perm, c->prop, NULL, &control));

        // continue if indicate is not supported
        if (!(c->prop & ESP_GATT_CHAR_PROP_BIT_INDICATE)) {
          continue;
        }

        // prepare client config descriptor uuid
        esp_bt_uuid_t ccd_uuid = {.len = ESP_UUID_LEN_16, .uuid.uuid16 = ESP_GATT_UUID_CHAR_CLIENT_CONFIG};

        // prepare client config descriptor value
        uint8_t ccd_value[2] = {0x02, 0x00};

        // prepare client config descriptor attribute
        esp_attr_value_t ccd_attr = {.attr_len = 2, .attr_max_len = 2, .attr_value = ccd_value};

        // prepare client config descriptor control
        esp_attr_control_t ccd_control = {.auto_rsp = ESP_GATT_AUTO_RSP};

        // add client config descriptor
        ESP_ERROR_CHECK(esp_ble_gatts_add_char_descr(naos_ble_gatts_profile.service_handle, &ccd_uuid, perm, &ccd_attr,
                                                     &ccd_control));
      }

      break;
    }

    // handle added characteristic event
    case ESP_GATTS_ADD_CHAR_EVT: {
      // check status
      ESP_ERROR_CHECK(p->add_char.status);

      // iterate through all characteristics
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        // get pointer of current characteristic
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

        // skip if uuid does not match
        if (memcmp(p->add_char.char_uuid.uuid.uuid128, c->_uuid.uuid.uuid128, ESP_UUID_LEN_128) != 0) {
          continue;
        }

        // save attribute handle
        c->handle = p->add_char.attr_handle;

        // set initialization bit if this is the last characteristic
        if (j + 1 == NAOS_BLE_NUM_CHARS) {
          naos_trigger(naos_ble_signal, 1, false);
        }

        // exit loop
        break;
      }

      break;
    }

    // handle added descriptor event
    case ESP_GATTS_ADD_CHAR_DESCR_EVT: {
      // check status
      ESP_ERROR_CHECK(p->add_char_descr.status);

      break;
    }

    // handle client connect event
    case ESP_GATTS_CONNECT_EVT: {
      // mark connection
      naos_ble_conns[p->connect.conn_id].id = p->connect.conn_id;
      naos_ble_conns[p->connect.conn_id].connected = true;
      naos_ble_conns[p->connect.conn_id].locked = strlen(naos_get("device-password")) > 0;

      // restart advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    // handle characteristic read event
    case ESP_GATTS_READ_EVT: {
      // stop immediately if no response is needed
      if (!p->read.need_rsp) {
        break;
      }

      // get connection
      naos_ble_conn_t *conn = &naos_ble_conns[p->read.conn_id];

      // iterate through all characteristics
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        // get pointer of current characteristic
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

        // skip if handles do not match
        if (p->read.handle != c->handle) {
          continue;
        }

        // check if characteristic is readable
        if (!(c->prop & ESP_GATT_CHAR_PROP_BIT_READ)) {
          // send error response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->read.conn_id, p->read.trans_id, ESP_GATT_READ_NOT_PERMIT, NULL));
          NAOS_UNLOCK(naos_ble_mutex);
          return;
        }

        // prepare response
        esp_gatt_rsp_t rsp = {0};
        rsp.attr_value.handle = c->handle;

        // handle characteristic
        char *value = NULL;
        if (c == &naos_ble_char_lock) {
          value = strdup(conn->locked ? "locked" : "unlocked");
        } else if (c == &naos_ble_char_list) {
          value = naos_params_list(conn->mode | (conn->locked ? NAOS_PUBLIC : 0));
        } else if (c == &naos_ble_char_select) {
          if (conn->param != NULL) {
            value = strdup(conn->param->name);
          }
        } else if (c == &naos_ble_char_value) {
          if (conn->param != NULL) {
            value = strdup(naos_get(conn->param->name));
          }
        }

        // set value
        if (value != NULL) {
          strcpy((char *)rsp.attr_value.value, value);
          rsp.attr_value.len = (uint16_t)strlen(value);
          free(value);
        }

        // send response
        ESP_ERROR_CHECK(esp_ble_gatts_send_response(i, p->read.conn_id, p->read.trans_id, ESP_GATT_OK, &rsp));

        // exit loop
        break;
      }

      break;
    }

    // handle response event
    case ESP_GATTS_RESPONSE_EVT: {
      // check status
      ESP_ERROR_CHECK(p->rsp.status);

      break;
    }

    // handle confirm event
    case ESP_GATTS_CONF_EVT: {
      // check status
      ESP_ERROR_CHECK(p->conf.status);

      break;
    }

    // handle characteristic write event
    case ESP_GATTS_WRITE_EVT: {
      // get connection
      naos_ble_conn_t *conn = &naos_ble_conns[p->write.conn_id];

      // iterate through all characteristics
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        // get pointer of current characteristic
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

        // skip if handles do not match
        if (p->write.handle != c->handle) {
          continue;
        }

        // check if characteristic is writable
        if (!(c->prop & ESP_GATT_CHAR_PROP_BIT_WRITE)) {
          // send error response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_WRITE_NOT_PERMIT, NULL));
          NAOS_UNLOCK(naos_ble_mutex);
          return;
        }

        // check attribute length and return if it exceeds
        if (p->write.len > c->max_write_len) {
          // send error response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_INVALID_ATTR_LEN, NULL));
          NAOS_UNLOCK(naos_ble_mutex);
          return;
        }

        // allocate value
        char *value = malloc(p->write.len + 1);
        memcpy(value, (char *)p->write.value, p->write.len);
        value[p->write.len] = '\0';

        // handle characteristic
        if (c == &naos_ble_char_lock) {
          if (conn->locked && strcmp(value, naos_get("device-password")) == 0) {
            conn->locked = false;
          }
        } else if (c == &naos_ble_char_list) {
          if (strcmp(value, "system") == 0) {
            conn->mode = NAOS_SYSTEM;
          } else if (strcmp(value, "application") == 0) {
            conn->mode = NAOS_APPLICATION;
          } else {
            conn->mode = 0;
          }
        } else if (c == &naos_ble_char_select) {
          naos_param_t *param = naos_lookup(value);
          if (param != NULL && (!conn->locked || (param->mode & NAOS_PUBLIC) != 0)) {
            conn->param = param;
          }
        } else if (c == &naos_ble_char_value) {
          if (conn->param != NULL && (conn->param->mode & NAOS_LOCKED) == 0) {
            naos_set(conn->param->name, value);
          }
        }

        // free value
        free(value);

        // send response if requested
        if (p->write.need_rsp) {
          ESP_ERROR_CHECK(esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_OK, NULL));
        }

        // exit loop
        break;
      }

      break;
    }

    // handle client disconnect event
    case ESP_GATTS_DISCONNECT_EVT: {
      // mark connection
      naos_ble_conns[p->disconnect.conn_id].id = 0;
      naos_ble_conns[p->disconnect.conn_id].connected = false;
      naos_ble_conns[p->connect.conn_id].locked = false;

      // restart advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    default: {
      // ESP_LOGI(NAOS_LOG_TAG, "unhandled GATTS event: %d", e);
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_ble_mutex);
}

static void naos_ble_param_handler(naos_param_t *param) {
  // acquire mutex
  NAOS_LOCK(naos_ble_mutex);

  // send indicate to all unlocked connections
  for (int j = 0; j < NAOS_BLE_MAX_CONNECTIONS; j++) {
    naos_ble_conn_t *conn = &naos_ble_conns[j];
    if (conn->connected && !conn->locked) {
      ESP_ERROR_CHECK(esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, j, naos_ble_char_update.handle,
                                                  (uint16_t)strlen(param->name), (uint8_t *)param->name, false));
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_ble_mutex);
}

static void ble_params(naos_param_t *param) {
  // update device name if changed
  if (strcmp(param->name, "device-name") == 0) {
    esp_ble_gap_set_device_name(param->value);
  }
}

void naos_ble_init() {
  // create mutex
  naos_ble_mutex = naos_mutex();

  // create even group
  naos_ble_signal = naos_signal();

  // iterate through all characteristics
  for (int i = 0; i < NAOS_BLE_NUM_CHARS; i++) {
    // get pointer of current characteristic
    naos_ble_gatts_char_t *c = naos_ble_gatts_chars[i];

    // setup uuid
    c->_uuid.len = ESP_UUID_LEN_128;
    memcpy(c->_uuid.uuid.uuid128, c->uuid, ESP_UUID_LEN_128);
  }

  // add primary service uuid to advertisement
  naos_ble_adv_data.service_uuid_len = ESP_UUID_LEN_128;
  naos_ble_adv_data.p_service_uuid = naos_ble_gatts_profile.service_id.id.uuid.uuid.uuid128;

  // initialize controller
  esp_bt_controller_config_t cfg = BT_CONTROLLER_INIT_CONFIG_DEFAULT();
  esp_bt_controller_init(&cfg);

  // enable bluetooth
  ESP_ERROR_CHECK(esp_bt_controller_enable(ESP_BT_MODE_BTDM));

  // initialize bluedroid stack
  ESP_ERROR_CHECK(esp_bluedroid_init());

  // enable bluedroid stack
  ESP_ERROR_CHECK(esp_bluedroid_enable());

  // register gatts handler
  ESP_ERROR_CHECK(esp_ble_gatts_register_callback(naos_ble_gatts_handler));

  // register gap handler
  ESP_ERROR_CHECK(esp_ble_gap_register_callback(naos_ble_gap_handler));

  // configure profile
  naos_ble_gatts_profile.interface = ESP_GATT_IF_NONE;
  naos_ble_gatts_profile.service_id.is_primary = true;
  naos_ble_gatts_profile.service_id.id.inst_id = 0;

  // set uuid
  naos_ble_gatts_profile.service_id.id.uuid.len = ESP_UUID_LEN_128;
  memcpy(naos_ble_gatts_profile.service_id.id.uuid.uuid.uuid128, naos_ble_gatts_profile.uuid, ESP_UUID_LEN_128);

  // register application
  esp_ble_gatts_app_register(0x55);

  // set device name
  esp_ble_gap_set_device_name(naos_get("device-name"));

  // subscribe params
  naos_params_subscribe(ble_params);

  // wait for initialization to complete
  naos_await(naos_ble_signal, 1, false);

  // handle parameters
  naos_params_subscribe(naos_ble_param_handler);
}
