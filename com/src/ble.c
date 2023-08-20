#include <naos/ble.h>
#include <naos/sys.h>
#include <naos/msg.h>

#include <esp_bt.h>
#include <esp_bt_defs.h>
#include <esp_bt_main.h>
#include <esp_gap_ble_api.h>
#include <esp_gatt_defs.h>
#include <esp_gatts_api.h>
#include <esp_gatt_common_api.h>
#include <esp_bt_device.h>
#include <string.h>

#include "naos.h"
#include "params.h"
#include "utils.h"
#include "update.h"

typedef struct {
  uint16_t id;
  bool connected;
  bool locked;
  naos_mode_t mode;
  naos_param_t *param;
} naos_ble_conn_t;

static naos_signal_t naos_ble_signal;

static esp_ble_adv_params_t naos_ble_adv_params = {
    .adv_int_min = 0x20,  // 20 ms
    .adv_int_max = 0x40,  // 40 ms
    .adv_type = ADV_TYPE_IND,
    .own_addr_type = BLE_ADDR_TYPE_PUBLIC,
    .channel_map = ADV_CHNL_ALL,
    .adv_filter_policy = ADV_FILTER_ALLOW_SCAN_ANY_CON_ANY,
};

static esp_ble_adv_data_t naos_ble_adv_data = {
    .include_name = true,
    .flag = ESP_BLE_ADV_FLAG_GEN_DISC | ESP_BLE_ADV_FLAG_BREDR_NOT_SPT,
};

static struct {
  uint8_t uuid[16];
  // ---
  esp_gatt_if_t interface;
  esp_gatt_srvc_id_t service_id;
  uint16_t service_handle;
} naos_ble_gatts_profile = {
    .uuid = {0xB5, 0x33, 0x50, 0x9D, 0xEE, 0xFF, 0x03, 0x81, 0x4F, 0x4E, 0x61, 0x48, 0x1B, 0xBA, 0x2F, 0x63},
};

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
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR,
    .max_write_len = 32};

static naos_ble_gatts_char_t naos_ble_char_list = {
    .uuid = {0x65, 0xa6, 0x6e, 0x1a, 0x95, 0x7d, 0x48, 0xdf, 0x8b, 0xb7, 0x1b, 0x23, 0xd1, 0x89, 0x22, 0xac},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR,
    .max_write_len = 32};

static naos_ble_gatts_char_t naos_ble_char_select = {
    .uuid = {0xcd, 0xba, 0xd4, 0x6e, 0x8d, 0xf8, 0x40, 0x42, 0xbe, 0xcc, 0x6f, 0x40, 0x6d, 0x70, 0xc9, 0xcf},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR,
    .max_write_len = 64};

static naos_ble_gatts_char_t naos_ble_char_value = {
    .uuid = {0xb3, 0x71, 0x1e, 0xb0, 0x84, 0x68, 0x41, 0x20, 0x99, 0x7e, 0xe1, 0x8e, 0x46, 0x54, 0xca, 0x01},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR,
    .max_write_len = 512};

static naos_ble_gatts_char_t naos_ble_char_update = {
    .uuid = {0x26, 0x17, 0x8c, 0xbc, 0x61, 0x7a, 0x4a, 0x9c, 0xa2, 0x22, 0x04, 0x07, 0xcf, 0xfd, 0xbf, 0x87},
    .prop = ESP_GATT_CHAR_PROP_BIT_INDICATE};

static naos_ble_gatts_char_t naos_ble_char_flash = {
    .uuid = {0x90, 0x13, 0x99, 0x4c, 0xfe, 0xa1, 0x41, 0x53, 0x87, 0x16, 0xa9, 0x9a, 0xa1, 0x4d, 0x11, 0x6c},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR |
            ESP_GATT_CHAR_PROP_BIT_INDICATE,
    .max_write_len = 512,
};

static naos_ble_gatts_char_t naos_ble_char_msg = {
    .uuid = {0xf3, 0x30, 0x41, 0x63, 0xf3, 0x37, 0x45, 0xc9, 0xad, 0x00, 0x1b, 0xa6, 0x4b, 0x74, 0x60, 0x03},
    .prop = ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR | ESP_GATT_CHAR_PROP_BIT_INDICATE,
    .max_write_len = 512,
};

#define NAOS_BLE_NUM_CHARS 7

#if defined(CONFIG_BTDM_CTRL_BLE_MAX_CONN_EFF)
#define NAOS_BLE_MAX_CONNECTIONS CONFIG_BTDM_CTRL_BLE_MAX_CONN_EFF
#elif defined(CONFIG_BT_ACL_CONNECTIONS)
#define NAOS_BLE_MAX_CONNECTIONS CONFIG_BT_ACL_CONNECTIONS
#else
#define NAOS_BLE_MAX_CONNECTIONS 1
#endif

static naos_ble_gatts_char_t *naos_ble_gatts_chars[NAOS_BLE_NUM_CHARS] = {
    &naos_ble_char_lock,   &naos_ble_char_list,  &naos_ble_char_select, &naos_ble_char_value,
    &naos_ble_char_update, &naos_ble_char_flash, &naos_ble_char_msg,
};

static naos_ble_conn_t naos_ble_conns[NAOS_BLE_MAX_CONNECTIONS];
static naos_ble_conn_t *naos_ble_flash_conn = NULL;
static bool naos_ble_flash_ready = false;
static uint8_t naos_ble_msg_channel_id = 0;

static void naos_ble_update(naos_update_event_t event) {
  // skip non-ready events
  if (event != NAOS_UPDATE_READY) {
    return;
  }

  // set flag
  naos_ble_flash_ready = true;

  // indicate readiness if still connected
  if (naos_ble_flash_conn->connected) {
    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, naos_ble_flash_conn->id,
                                                              naos_ble_char_flash.handle, 1, (uint8_t *)"1", false));
  }
}

static void naos_ble_gap_handler(esp_gap_ble_cb_event_t e, esp_ble_gap_cb_param_t *p) {
  switch (e) {
    case ESP_GAP_BLE_ADV_DATA_SET_COMPLETE_EVT: {
      // begin advertising
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
  // pre-check for registration event
  if (e == ESP_GATTS_REG_EVT) {
    // check status
    ESP_ERROR_CHECK(p->reg.status);

    // store GATTS interface after registration
    naos_ble_gatts_profile.interface = i;
  }

  // return immediately if event is not general or does not belong to our interface
  if (i != ESP_GATT_IF_NONE && i != naos_ble_gatts_profile.interface) {
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
      naos_ble_conns[p->connect.conn_id].locked = strlen(naos_get_s("device-password")) > 0;

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
          return;
        }

        // prepare response
        esp_gatt_rsp_t rsp = {0};
        rsp.attr_value.handle = c->handle;

        // handle characteristic
        if (c == &naos_ble_char_lock) {
          strcpy((char *)rsp.attr_value.value, conn->locked ? "locked" : "unlocked");
        } else if (c == &naos_ble_char_list) {
          char *list = naos_params_list(conn->mode | (conn->locked ? NAOS_PUBLIC : 0));
          strcpy((char *)rsp.attr_value.value, list);
          free(list);
        } else if (c == &naos_ble_char_select) {
          if (conn->param != NULL) {
            strcpy((char *)rsp.attr_value.value, conn->param->name);
          }
        } else if (c == &naos_ble_char_value) {
          if (conn->param != NULL) {
            memcpy(rsp.attr_value.value, conn->param->current.buf, conn->param->current.len);
            rsp.attr_value.len = conn->param->current.len;
          }
        } else if (c == &naos_ble_char_flash) {
          if (!conn->locked && naos_ble_flash_conn == conn) {
            strcpy((char *)rsp.attr_value.value, naos_ble_flash_ready ? "1" : "0");
          }
        }

        // set string length
        if (rsp.attr_value.len == 0 && rsp.attr_value.value[0] != 0) {
          rsp.attr_value.len = strlen((char *)rsp.attr_value.value);
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
        if (!(c->prop & (ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR))) {
          // send error response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_WRITE_NOT_PERMIT, NULL));
          return;
        }

        // check attribute length and return if it exceeds
        if (p->write.len > c->max_write_len) {
          // send error response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_INVALID_ATTR_LEN, NULL));
          return;
        }

        // prepare status
        esp_gatt_status_t status = ESP_GATT_OK;

        // handle characteristic
        if (c == &naos_ble_char_lock) {
          if (conn->locked && naos_equal(p->write.value, p->write.len, naos_get_s("device-password"))) {
            conn->locked = false;
          }
        } else if (c == &naos_ble_char_list) {
          if (naos_equal(p->write.value, p->write.len, "system")) {
            conn->mode = NAOS_SYSTEM;
          } else if (naos_equal(p->write.value, p->write.len, "application")) {
            conn->mode = NAOS_APPLICATION;
          } else {
            conn->mode = 0;
          }
        } else if (c == &naos_ble_char_select) {
          char *value = (char *)naos_copy(p->write.value, p->write.len);
          naos_param_t *param = naos_lookup(value);
          free(value);
          if (param != NULL && (!conn->locked || (param->mode & NAOS_PUBLIC) != 0)) {
            conn->param = param;
          }
        } else if (c == &naos_ble_char_value) {
          if (conn->param != NULL && (conn->param->mode & NAOS_LOCKED) == 0) {
            naos_set(conn->param->name, p->write.value, p->write.len);
          }
        } else if (c == &naos_ble_char_flash) {
          if (!conn->locked && p->write.len > 0) {
            switch (p->write.value[0]) {
              case 'b': {  // begin
                size_t size = strtoul((char *)(p->write.value + 1), NULL, 10);
                naos_ble_flash_conn = conn;
                naos_ble_flash_ready = false;
                naos_update_begin(size, naos_ble_update);
                break;
              }
              case 'w': {  // write
                naos_update_write((uint8_t *)(p->write.value + 1), p->write.len - 1);
                break;
              }
              case 'f': {  // finish
                naos_update_finish();
                naos_ble_flash_conn = NULL;
                naos_ble_flash_ready = false;
                break;
              }
            }
          }
        } else if (c == &naos_ble_char_msg) {
          if (!conn->locked && p->write.len > 0) {
            bool ok = naos_msg_dispatch(naos_ble_msg_channel_id, p->write.value, p->write.len, conn);
            if (!ok) {
              status = ESP_GATT_UNKNOWN_ERROR;
            }
          }
        }

        // send response if requested
        if (p->write.need_rsp) {
          ESP_ERROR_CHECK(esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, status, NULL));
        }

        // exit loop
        break;
      }

      break;
    }

    // handle client disconnect event
    case ESP_GATTS_DISCONNECT_EVT: {
      // get connection
      naos_ble_conn_t *conn = &naos_ble_conns[p->disconnect.conn_id];

      // clear connection
      *conn = (naos_ble_conn_t){0};

      // restart advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    default: {
      // ESP_LOGI(NAOS_LOG_TAG, "unhandled GATTS event: %d", e);
    }
  }
}

static void naos_ble_set_name() {
  // prepare name
  const char *name = naos_get_s("device-name");

  // use device type if absent
  if (strlen(name) == 0) {
    name = naos_config()->device_type;
  }

  // cap name to not exceed adv packet
  if (strlen(name) > 8) {
    char copy[9] = {0};
    strncpy(copy, name, 8);
    name = copy;
  }

  // set name
  ESP_ERROR_CHECK(esp_bt_dev_set_device_name(name));
  ESP_ERROR_CHECK(esp_ble_gap_set_device_name(name));
  ESP_ERROR_CHECK(esp_ble_gap_config_adv_data(&naos_ble_adv_data));
}

static void naos_ble_param_handler(naos_param_t *param) {
  // update device name if changed
  if (strcmp(param->name, "device-name") == 0) {
    naos_ble_set_name();
  }

  // send indicate to all unlocked connections
  for (int j = 0; j < NAOS_BLE_MAX_CONNECTIONS; j++) {
    naos_ble_conn_t *conn = &naos_ble_conns[j];
    if (conn->connected && !conn->locked) {
      ESP_ERROR_CHECK_WITHOUT_ABORT(
          esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, conn->id, naos_ble_char_update.handle,
                                      (uint16_t)strlen(param->name), (uint8_t *)param->name, false));
    }
  }
}

static bool naos_ble_msg_send(const uint8_t *data, size_t len, void *ctx) {
  // get conn
  naos_ble_conn_t *conn = ctx;
  if (!conn->connected || conn->locked) {
    return false;
  }

  // wait up to 1 second until packet can be sent
  int attempts = 0;
  while (esp_ble_get_cur_sendable_packets_num(conn->id) == 0 && attempts++ < 1000) {
    naos_delay(1);
  }

  // send indicate
  esp_err_t err = esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, conn->id, naos_ble_char_msg.handle,
                                              (uint16_t)len, (uint8_t *)data, false);
  ESP_ERROR_CHECK_WITHOUT_ABORT(err);

  return err == ESP_OK;
}

void naos_ble_init(naos_ble_config_t cfg) {
  // Note: The BLE subsystem is not protected by a mutex to prevent deadlocks of
  // the bluetooth task. Most BLE calls block until the bluetooth task replies,
  // but other work items issues beforehand may call into our code before the
  // reply is issued. Therefore, we opt for a careful lock-free usage.

  // create signal
  naos_ble_signal = naos_signal();

  // initialize bluetooth
  if (!cfg.skip_bt_init) {
    // enable controller
    esp_bt_controller_config_t bt_cfg = BT_CONTROLLER_INIT_CONFIG_DEFAULT();
    ESP_ERROR_CHECK(esp_bt_controller_init(&bt_cfg));
#if defined(CONFIG_BTDM_CTRL_MODE_BTDM)
    ESP_ERROR_CHECK(esp_bt_controller_enable(ESP_BT_MODE_BTDM));
#else
    ESP_ERROR_CHECK(esp_bt_controller_enable(ESP_BT_MODE_BLE));
#endif

    // enable bluedroid
    ESP_ERROR_CHECK(esp_bluedroid_init());
    ESP_ERROR_CHECK(esp_bluedroid_enable());
  }

  // prepare characteristic UUIDs
  for (int i = 0; i < NAOS_BLE_NUM_CHARS; i++) {
    naos_ble_gatts_char_t *c = naos_ble_gatts_chars[i];
    c->_uuid.len = ESP_UUID_LEN_128;
    memcpy(c->_uuid.uuid.uuid128, c->uuid, ESP_UUID_LEN_128);
  }

  // add primary service UUID to advertisement
  naos_ble_adv_data.service_uuid_len = ESP_UUID_LEN_128;
  naos_ble_adv_data.p_service_uuid = naos_ble_gatts_profile.service_id.id.uuid.uuid.uuid128;

  // register GATTS handler
  ESP_ERROR_CHECK(esp_ble_gatts_register_callback(naos_ble_gatts_handler));

  // register GAP handler
  ESP_ERROR_CHECK(esp_ble_gap_register_callback(naos_ble_gap_handler));

  // configure GATTS profile
  naos_ble_gatts_profile.interface = ESP_GATT_IF_NONE;
  naos_ble_gatts_profile.service_id.is_primary = true;
  naos_ble_gatts_profile.service_id.id.inst_id = 0;
  naos_ble_gatts_profile.service_id.id.uuid.len = ESP_UUID_LEN_128;
  memcpy(naos_ble_gatts_profile.service_id.id.uuid.uuid.uuid128, naos_ble_gatts_profile.uuid, ESP_UUID_LEN_128);

  // register application
  ESP_ERROR_CHECK(esp_ble_gatts_app_register(0x55));
  naos_await(naos_ble_signal, 1, false);

  // set device name
  naos_ble_set_name();

  // handle parameters
  naos_params_subscribe(naos_ble_param_handler);

  // register channel
  naos_ble_msg_channel_id = naos_msg_register((naos_msg_channel_t){
      .name = "ble",
      .mtu = 512,
      .send = naos_ble_msg_send,
  });
}
