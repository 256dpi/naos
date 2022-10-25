#include <freertos/FreeRTOS.h>

#include <esp_bt.h>
#include <esp_bt_defs.h>
#include <esp_bt_main.h>
#include <esp_gap_ble_api.h>
#include <esp_gatt_defs.h>
#include <esp_gatts_api.h>
#include <freertos/event_groups.h>
#include <freertos/semphr.h>
#include <string.h>

#include "naos.h"
#include "utils.h"
#include "config.h"
#include "settings.h"

#define NAOS_BLE_INITIALIZED_BIT (1 << 0)
#define NAOS_BLE_MAX_CONNECTIONS CONFIG_BT_ACL_CONNECTIONS

typedef struct {
  uint16_t id;
  bool connected;
  bool locked;
  naos_setting_t setting;
  naos_param_t *param;
} naos_ble_conn_t;

static SemaphoreHandle_t naos_ble_mutex;
static EventGroupHandle_t naos_ble_signal;

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
    .min_interval = 0x20,
    .max_interval = 0x40,
    .appearance = 0x00,
    .manufacturer_len = 0,
    .p_manufacturer_data = NULL,
    .service_data_len = 0,
    .p_service_data = NULL,
    .service_uuid_len = 0,
    .p_service_uuid = NULL,
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

static naos_ble_gatts_char_t naos_ble_char_description = {
    .uuid = {0x26, 0x17, 0x8c, 0xbc, 0x61, 0x7a, 0x4a, 0x9c, 0xa2, 0x22, 0x04, 0x07, 0xcf, 0xfd, 0xbf, 0x87},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_INDICATE};

static naos_ble_gatts_char_t naos_ble_char_lock = {
    .uuid = {0x91, 0xb5, 0x2e, 0x90, 0xd5, 0x07, 0x4d, 0x68, 0x9b, 0x23, 0x84, 0x40, 0xa4, 0xfb, 0xa5, 0xf7},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_INDICATE,
    .max_write_len = 32};

static naos_ble_gatts_char_t naos_ble_char_settings_list = {
    .uuid = {0x5d, 0x28, 0x5f, 0xe5, 0x88, 0x5c, 0x4c, 0xbb, 0xa9, 0x80, 0xeb, 0xb5, 0x2c, 0xe4, 0xae, 0xde},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ};

static naos_ble_gatts_char_t naos_ble_char_settings_select = {
    .uuid = {0x7b, 0xf3, 0xcc, 0x62, 0x7a, 0x2d, 0x48, 0xb8, 0xbd, 0x87, 0x9b, 0x33, 0xbb, 0x99, 0x7f, 0xa9},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 64};

static naos_ble_gatts_char_t naos_ble_char_settings_value = {
    .uuid = {0x7e, 0x94, 0xc9, 0x25, 0xff, 0x0a, 0x4a, 0x61, 0xa0, 0x50, 0xe1, 0xe7, 0xcb, 0xbe, 0xbe, 0xc8},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 128};

static naos_ble_gatts_char_t naos_ble_char_command = {
    .uuid = {0x29, 0x92, 0x52, 0xd1, 0xe5, 0xba, 0x40, 0xb4, 0x91, 0x88, 0x82, 0x7f, 0x43, 0x4d, 0x63, 0xf1},
    .prop = ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 32};

static naos_ble_gatts_char_t naos_ble_char_params_list = {
    .uuid = {0x65, 0xa6, 0x6e, 0x1a, 0x95, 0x7d, 0x48, 0xdf, 0x8b, 0xb7, 0x1b, 0x23, 0xd1, 0x89, 0x22, 0xac},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ};

static naos_ble_gatts_char_t naos_ble_char_params_select = {
    .uuid = {0xcd, 0xba, 0xd4, 0x6e, 0x8d, 0xf8, 0x40, 0x42, 0xbe, 0xcc, 0x6f, 0x40, 0x6d, 0x70, 0xc9, 0xcf},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 64};

static naos_ble_gatts_char_t naos_ble_char_params_value = {
    .uuid = {0xb3, 0x71, 0x1e, 0xb0, 0x84, 0x68, 0x41, 0x20, 0x99, 0x7e, 0xe1, 0x8e, 0x46, 0x54, 0xca, 0x01},
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_write_len = 128};

#define NAOS_BLE_NUM_CHARS 9

static naos_ble_gatts_char_t *naos_ble_gatts_chars[NAOS_BLE_NUM_CHARS] = {
    &naos_ble_char_description,     &naos_ble_char_lock,           &naos_ble_char_settings_list,
    &naos_ble_char_settings_select, &naos_ble_char_settings_value, &naos_ble_char_command,
    &naos_ble_char_params_list,     &naos_ble_char_params_select,  &naos_ble_char_params_value};

static naos_ble_conn_t naos_ble_conns[NAOS_BLE_MAX_CONNECTIONS];

static void naos_ble_gap_event_handler(esp_gap_ble_cb_event_t e, esp_ble_gap_cb_param_t *p) {
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

static void naos_ble_gatts_event_handler(esp_gatts_cb_event_t e, esp_gatt_if_t i, esp_ble_gatts_cb_param_t *p) {
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
      // set device name
      ESP_ERROR_CHECK(esp_ble_gap_set_device_name("naos"));

      // set advertisement config
      ESP_ERROR_CHECK(esp_ble_gap_config_adv_data(&naos_ble_adv_data));

      // prepare total with on handle for the service
      uint16_t total_handles = 1;

      // iterate through all characteristics
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        // get pointer of current characteristic
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

        // add characteristic handles
        total_handles += 2;

        // add one handle for client descriptor
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

        // move on if uuid does not match
        if (memcmp(p->add_char.char_uuid.uuid.uuid128, c->_uuid.uuid.uuid128, ESP_UUID_LEN_128) != 0) {
          continue;
        }

        // save attribute handle
        c->handle = p->add_char.attr_handle;

        // set initialization bit if this is the last characteristic
        if (j + 1 == NAOS_BLE_NUM_CHARS) {
          xEventGroupSetBits(naos_ble_signal, NAOS_BLE_INITIALIZED_BIT);
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
      naos_ble_conns[p->connect.conn_id].locked = naos_config()->password != NULL;

      break;
    }

    // handle characteristic read event
    case ESP_GATTS_READ_EVT: {
      // break immediately if no response is needed
      if (!p->read.need_rsp) {
        break;
      }

      // get connection
      naos_ble_conn_t *conn = &naos_ble_conns[p->read.conn_id];

      // iterate through all characteristics
      for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
        // get pointer of current characteristic
        naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

        // move on if handles do not match
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
        esp_gatt_rsp_t rsp;
        memset(&rsp, 0, sizeof(esp_gatt_rsp_t));

        // set handle
        rsp.attr_value.handle = c->handle;

        // prepare value
        char *value = NULL;

        // handle characteristic
        if (c == &naos_ble_char_description) {
          value = naos_config_describe(conn->locked);
        } else if (c == &naos_ble_char_lock) {
          value = strdup(conn->locked ? "locked" : "unlocked");
        } else if (!conn->locked) {
          if (c == &naos_ble_char_settings_list) {
            value = naos_config_list_settings();
          } else if (c == &naos_ble_char_settings_select) {
            if (conn->setting != 0) {
              value = strdup(naos_setting_to_key(conn->setting));
            }
          } else if (c == &naos_ble_char_settings_value) {
            value = naos_config_read_setting(naos_setting_to_key(conn->setting));
          } else if (c == &naos_ble_char_command) {
            // ignore
          } else if (c == &naos_ble_char_params_list) {
            value = naos_config_list_params();
          } else if (c == &naos_ble_char_params_select) {
            if (conn->param != NULL) {
              value = strdup(conn->param->name);
            }
          } else if (c == &naos_ble_char_params_value) {
            value = naos_config_read_param(conn->param->name);
          }
        }

        // TODO: Check offset

        // set value
        if (value != NULL) {
          strcpy((char *)rsp.attr_value.value, value);
          rsp.attr_value.len = (uint16_t)strlen(value);
        }

        // free value
        if (value != NULL) {
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

        // move on if handles do not match
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

        // TODO: Check offset

        // allocate value
        char *value = malloc(p->write.len + 1);
        memcpy(value, (char *)p->write.value, p->write.len);
        value[p->write.len] = '\0';

        // prepare flag
        bool indicate_unlock = false;

        // handle unlocks directly
        if (c == &naos_ble_char_description) {
          // ignore
        } else if (c == &naos_ble_char_lock) {
          if (conn->locked && strcmp(value, naos_config()->password) == 0) {
            conn->locked = false;
            indicate_unlock = true;
          }
        } else if (!conn->locked) {
          if (c == &naos_ble_char_settings_list) {
            // ignore
          } else if (c == &naos_ble_char_settings_select) {
            conn->setting = naos_setting_from_key(value);
          } else if (c == &naos_ble_char_settings_value) {
            naos_config_write_setting(naos_setting_to_key(conn->setting), value);
          } else if (c == &naos_ble_char_command) {
            naos_config_execute(value);
          } else if (c == &naos_ble_char_params_list) {
            // ignore
          } else if (c == &naos_ble_char_params_select) {
            conn->param = naos_lookup(value);
          } else if (c == &naos_ble_char_params_value) {
            naos_config_write_parm(conn->param->name, value);
          }
        }

        // free value
        free(value);

        // send response if requested
        if (p->write.need_rsp) {
          ESP_ERROR_CHECK(esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_OK, NULL));
        }

        // check lock indication
        if (indicate_unlock) {
          ESP_ERROR_CHECK(esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, p->write.conn_id,
                                                      naos_ble_char_lock.handle, 8, (uint8_t *)"unlocked", false));
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

static void naos_ble_notification_handler(naos_config_notification_t notification) {
  // acquire mutex
  NAOS_LOCK(naos_ble_mutex);

  // send indicate to all unlocked connections
  for (int j = 0; j < NAOS_BLE_MAX_CONNECTIONS; j++) {
    if (naos_ble_conns[j].connected && !naos_ble_conns[j].locked) {
      naos_ble_conn_t *conn = &naos_ble_conns[j];
      switch (notification) {
        case NAOS_CONFIG_NOTIFICATION_DESCRIPTION: {
          char *value = naos_config_describe(conn->locked);
          ESP_ERROR_CHECK(esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface, j,
                                                      naos_ble_char_description.handle, (uint16_t)strlen(value),
                                                      (uint8_t *)value, false));
          free(value);
        }
      }
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_ble_mutex);
}

void naos_ble_init() {
  // create mutex
  naos_ble_mutex = xSemaphoreCreateMutex();

  // create even group
  naos_ble_signal = xEventGroupCreate();

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

  // register gatts callback
  ESP_ERROR_CHECK(esp_ble_gatts_register_callback(naos_ble_gatts_event_handler));

  // register gap callback
  ESP_ERROR_CHECK(esp_ble_gap_register_callback(naos_ble_gap_event_handler));

  // configure profile
  naos_ble_gatts_profile.interface = ESP_GATT_IF_NONE;
  naos_ble_gatts_profile.service_id.is_primary = true;
  naos_ble_gatts_profile.service_id.id.inst_id = 0;

  // set uuid
  naos_ble_gatts_profile.service_id.id.uuid.len = ESP_UUID_LEN_128;
  memcpy(naos_ble_gatts_profile.service_id.id.uuid.uuid.uuid128, naos_ble_gatts_profile.uuid, ESP_UUID_LEN_128);

  // register application
  esp_ble_gatts_app_register(0x55);

  // wait for initialization to complete
  xEventGroupWaitBits(naos_ble_signal, NAOS_BLE_INITIALIZED_BIT, pdFALSE, pdFALSE, portMAX_DELAY);

  // register notification handler
  naos_config_register(naos_ble_notification_handler);
}
