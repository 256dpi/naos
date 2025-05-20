#include <naos/ble.h>
#include <naos/sys.h>
#include <naos/msg.h>

#include <esp_log.h>
#include <esp_bt.h>
#include <esp_bt_defs.h>
#include <esp_bt_main.h>
#include <esp_gap_ble_api.h>
#include <esp_gatt_defs.h>
#include <esp_gatts_api.h>
#include <esp_gatt_common_api.h>
#include <string.h>

#include "params.h"
#include "utils.h"

typedef struct {
  uint16_t id;
  uint16_t mtu;
  bool congested;
  bool connected;
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
    .min_interval = 6,   // 7.5ms
    .max_interval = 12,  // 15ms
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

static naos_ble_gatts_char_t naos_ble_char_msg = {
    .uuid = {0xf3, 0x30, 0x41, 0x63, 0xf3, 0x37, 0x45, 0xc9, 0xad, 0x00, 0x1b, 0xa6, 0x4b, 0x74, 0x60, 0x03},
    .prop = ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR | ESP_GATT_CHAR_PROP_BIT_INDICATE,
    .max_write_len = 512,
};

#define NAOS_BLE_NUM_CHARS 1
#define NAOS_BLE_MAX_CONNECTIONS 8

static naos_ble_gatts_char_t *naos_ble_gatts_chars[NAOS_BLE_NUM_CHARS] = {
    &naos_ble_char_msg,
};

static naos_ble_config_t naos_ble_config = {0};
static naos_ble_conn_t naos_ble_conns[NAOS_BLE_MAX_CONNECTIONS];
static uint8_t naos_ble_msg_channel_id = 0;

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
        ESP_LOGE(NAOS_LOG_TAG, "naos_ble_gap_handler: failed to start advertisement (%d)", p->adv_data_raw_cmpl.status);
        break;
      }

      break;
    }

    case ESP_GAP_BLE_UPDATE_CONN_PARAMS_EVT: {
      // check status
      if (p->update_conn_params.status != ESP_BT_STATUS_SUCCESS) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_ble_gap_handler: failed to update connection parameters (%d)",
                 p->adv_data_raw_cmpl.status);
        break;
      }

      // log info
      ESP_LOGI(NAOS_LOG_TAG,
               "naos_ble_gap_handler: connection parameters updated (min_int=%d max_int=%d latency=%d conn_int=%d "
               "timeout=%d)",
               p->update_conn_params.min_int, p->update_conn_params.max_int, p->update_conn_params.latency,
               p->update_conn_params.conn_int, p->update_conn_params.timeout);
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
      // log info
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gatts_handler: new connection (id=%d interval=%d latency=%d timeout=%d)",
               p->connect.conn_id, p->connect.conn_params.interval, p->connect.conn_params.latency,
               p->connect.conn_params.timeout);

      // mark connection
      naos_ble_conns[p->connect.conn_id].id = p->connect.conn_id;
      naos_ble_conns[p->connect.conn_id].mtu = ESP_GATT_DEF_BLE_MTU_SIZE;
      naos_ble_conns[p->connect.conn_id].connected = true;

      // update connection params
      esp_ble_conn_update_params_t conn_params;
      memcpy(conn_params.bda, p->connect.remote_bda, sizeof(conn_params.bda));
      conn_params.min_int = 6;    // 7.5ms
      conn_params.max_int = 12;   // 15ms
      conn_params.latency = 0;    // no skips
      conn_params.timeout = 500;  // 5s
      ESP_ERROR_CHECK(esp_ble_gap_update_conn_params(&conn_params));

      // restart advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    // handle congestion event
    case ESP_GATTS_CONGEST_EVT: {
      // log info
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gatts_handler: congestion changed (id=%d congested=%d)", p->congest.conn_id,
               p->congest.congested);

      // get connection
      naos_ble_conn_t *conn = &naos_ble_conns[p->congest.conn_id];

      // set flag
      conn->congested = p->congest.congested;

      break;
    }

    // handle characteristic read event
    case ESP_GATTS_READ_EVT: {
      // stop immediately if no response is needed
      if (!p->read.need_rsp) {
        break;
      }

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
      ESP_ERROR_CHECK_WITHOUT_ABORT(p->rsp.status);

      break;
    }

    // handle MTU event
    case ESP_GATTS_MTU_EVT: {
      // log info
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gatts_handler: MTU changed to %d (conn=%d)", p->mtu.mtu, p->mtu.conn_id);

      // set MTU
      naos_ble_conns[p->mtu.conn_id].mtu = p->mtu.mtu;

      break;
    }

    // handle confirm event
    case ESP_GATTS_CONF_EVT: {
      // check status
      if (p->conf.status != ESP_GATT_OK) {
        ESP_LOGW(NAOS_LOG_TAG, "naos_ble_gatts_handler: failed to send indication (%d)", p->conf.status);
      }

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
        if (c == &naos_ble_char_msg) {
          if (p->write.len > 0) {
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
      // log info
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gatts_handler: lost connection (id=%d, reason=%d)", p->disconnect.conn_id,
               p->disconnect.reason);

      // clear connection
      naos_ble_conns[p->disconnect.conn_id] = (naos_ble_conn_t){0};

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
  ESP_ERROR_CHECK(esp_ble_gap_set_device_name(name));
  ESP_ERROR_CHECK(esp_ble_gap_config_adv_data(&naos_ble_adv_data));
}

static void naos_ble_param_handler(naos_param_t *param) {
  // update device name if changed
  if (strcmp(param->name, "device-name") == 0) {
    naos_ble_set_name();
  }
}

static uint16_t naos_ble_msg_mtu(void *ctx) {
  // get conn
  naos_ble_conn_t *conn = ctx;

  // return MTU
  return conn->mtu - 5;  // 5 bytes are reserved for the BLE stack
}

static bool naos_ble_msg_send(const uint8_t *data, size_t len, void *ctx) {
  for (int i = 0; i < 5; i++) {
    // get conn
    naos_ble_conn_t *conn = ctx;
    if (!conn->connected) {
      return false;
    }

    // wait up to a second until packets can be sent again
    int attempts = 0;
    while ((conn->congested || esp_ble_get_cur_sendable_packets_num(conn->id) < 5) && attempts++ < 1000) {
      naos_delay(1);
    }

    // send indicate
    esp_err_t err = esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, conn->id, naos_ble_char_msg.handle,
                                                (uint16_t)len, (uint8_t *)data, false);
    if (err != ESP_OK) {
      ESP_LOGW(NAOS_LOG_TAG, "naos_ble_msg_send: failed so send msg as notification (%d)", err);
      continue;
    }

    return true;
  }

  return false;
}

void naos_ble_init(naos_ble_config_t cfg) {
  // store config
  naos_ble_config = cfg;

  // Note: The BLE subsystem is not protected by a mutex to prevent deadlocks of
  // the bluetooth task. Most BLE calls block until the bluetooth task replies,
  // but other work items issues beforehand may call into our code before the
  // reply is issued. Therefore, we opt for a careful lock-free usage.

  // create signal
  naos_ble_signal = naos_signal();

  // initialize bluetooth
  if (!cfg.skip_bt_init) {
    // free some memory unless when using dual-mode
#if !defined(CONFIG_BTDM_CTRL_MODE_BTDM)
    ESP_ERROR_CHECK(esp_bt_controller_mem_release(ESP_BT_MODE_CLASSIC_BT));
#endif

    // enable controller
    esp_bt_controller_config_t bt_cfg = BT_CONTROLLER_INIT_CONFIG_DEFAULT();
    ESP_ERROR_CHECK(esp_bt_controller_init(&bt_cfg));
#if defined(CONFIG_BTDM_CTRL_MODE_BTDM)
    ESP_ERROR_CHECK(esp_bt_controller_enable(ESP_BT_MODE_BTDM));
#else
    ESP_ERROR_CHECK(esp_bt_controller_enable(ESP_BT_MODE_BLE));
#endif

    // enable bluedroid
    esp_bluedroid_config_t bld_cfg = {0};
    ESP_ERROR_CHECK(esp_bluedroid_init_with_cfg(&bld_cfg));
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
      .mtu = naos_ble_msg_mtu,
      .send = naos_ble_msg_send,
  });
}
