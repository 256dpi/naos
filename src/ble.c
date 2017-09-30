#include <bt.h>
#include <bta_api.h>
#include <esp_bt_defs.h>
#include <esp_bt_main.h>
#include <esp_gap_ble_api.h>
#include <esp_gatt_defs.h>
#include <esp_gatts_api.h>
#include <freertos/FreeRTOS.h>
#include <freertos/event_groups.h>
#include <freertos/semphr.h>
#include <nvs.h>
#include <string.h>

#include "ble.h"
#include "utils.h"

#define NAOS_BLE_INITIALIZED_BIT (1 << 0)

static EventGroupHandle_t naos_ble_init_event_group;

static SemaphoreHandle_t naos_ble_mutex;

static naos_ble_attribute_callback_t naos_ble_callback = NULL;

static nvs_handle naos_ble_nvs_handle;

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
  bool client_connected;
  uint16_t client_handle;
} naos_ble_gatts_profile = {
    .uuid = {0xB5, 0x33, 0x50, 0x9D, 0xEE, 0xFF, 0x03, 0x81, 0x4F, 0x4E, 0x61, 0x48, 0x1B, 0xBA, 0x2F, 0x63}};

typedef struct {
  naos_ble_id_t id;
  uint8_t uuid[16];
  const char *nvs_key;
  esp_gatt_char_prop_t prop;
  uint8_t max_length;
  // ---
  uint8_t *storage;
  uint8_t length;
  uint16_t handle;
  esp_bt_uuid_t _uuid;
} naos_ble_gatts_char_t;

static naos_ble_gatts_char_t naos_ble_char_wifi_ssid = {
    .id = NAOS_BLE_ID_WIFI_SSID,
    .uuid = {0x10, 0xA5, 0x75, 0x82, 0x56, 0xA3, 0x86, 0xBE, 0x90, 0x4C, 0x04, 0xCA, 0x27, 0xD3, 0x2D, 0x80},
    .nvs_key = "wifi-ssid",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_wifi_password = {
    .id = NAOS_BLE_ID_WIFI_PASSWORD,
    .uuid = {0x51, 0xC1, 0xB2, 0x8F, 0x49, 0xC3, 0x91, 0x97, 0xB7, 0x4C, 0x60, 0xF3, 0x61, 0x32, 0x88, 0xB3},
    .nvs_key = "wifi-password",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_mqtt_host = {
    .id = NAOS_BLE_ID_MQTT_HOST,
    .uuid = {0x57, 0xAC, 0x40, 0x5D, 0x35, 0x4A, 0x1F, 0xBE, 0xBC, 0x4E, 0x42, 0x45, 0xF2, 0xFF, 0x3F, 0x19},
    .nvs_key = "mqtt-host",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_mqtt_port = {
    .id = NAOS_BLE_ID_MQTT_PORT,
    .uuid = {0x55, 0x97, 0xD4, 0x82, 0x73, 0x42, 0x6F, 0x9A, 0x87, 0x47, 0x6C, 0x54, 0x4C, 0x76, 0x8A, 0xCB},
    .nvs_key = "mqtt-port",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 5};

static naos_ble_gatts_char_t naos_ble_char_mqtt_client_id = {
    .id = NAOS_BLE_ID_MQTT_CLIENT_ID,
    .uuid = {0x3B, 0x6C, 0x91, 0xBF, 0xA5, 0xC8, 0xA8, 0xBB, 0xF6, 0x4B, 0xCC, 0x65, 0x43, 0xE5, 0xC4, 0x08},
    .nvs_key = "mqtt-client-id",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_mqtt_username = {
    .id = NAOS_BLE_ID_MQTT_USERNAME,
    .uuid = {0x57, 0x02, 0xA0, 0x06, 0xD8, 0x72, 0xF5, 0x80, 0x9C, 0x44, 0xE9, 0x85, 0x9A, 0xA5, 0xB4, 0xAB},
    .nvs_key = "mqtt-username",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_mqtt_password = {
    .id = NAOS_BLE_ID_MQTT_PASSWORD,
    .uuid = {0x25, 0x19, 0xD4, 0x21, 0x55, 0x21, 0x52, 0x80, 0xC3, 0x4F, 0x58, 0x06, 0xB1, 0xB1, 0xEC, 0xC5},
    .nvs_key = "mqtt-password",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_device_type = {
    .id = NAOS_BLE_ID_DEVICE_TYPE,
    .uuid = {0x91, 0x91, 0x7B, 0x75, 0xA8, 0x8E, 0x83, 0x88, 0x08, 0x43, 0x6E, 0x19, 0x20, 0x31, 0xEA, 0x0C},
    .nvs_key = NULL,
    .prop = ESP_GATT_CHAR_PROP_BIT_READ,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_device_name = {
    .id = NAOS_BLE_ID_DEVICE_NAME,
    .uuid = {0x56, 0x0B, 0x6D, 0xEB, 0x3C, 0xAB, 0x6A, 0x92, 0xEA, 0x40, 0xF3, 0x28, 0x50, 0x78, 0x42, 0x25},
    .nvs_key = "device-name",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_base_topic = {
    .id = NAOS_BLE_ID_BASE_TOPIC,
    .uuid = {0xB4, 0xCA, 0x63, 0x8C, 0x9B, 0xD0, 0xA2, 0x8E, 0x38, 0x49, 0xF8, 0x9F, 0xA8, 0xE3, 0xB7, 0xEA},
    .nvs_key = "base-topic",
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 64};

static naos_ble_gatts_char_t naos_ble_char_connection_status = {
    .id = NAOS_BLE_ID_CONNECTION_STATUS,
    .uuid = {0x7C, 0xEB, 0x12, 0xD3, 0xF6, 0xD9, 0x00, 0xA2, 0x3C, 0x43, 0x50, 0x3A, 0xE0, 0x7C, 0x99, 0x59},
    .nvs_key = NULL,
    .prop = ESP_GATT_CHAR_PROP_BIT_READ | ESP_GATT_CHAR_PROP_BIT_INDICATE,
    .max_length = 32};

static naos_ble_gatts_char_t naos_ble_char_system_command = {
    .id = NAOS_BLE_ID_COMMAND,
    .uuid = {0xAB, 0xD0, 0x76, 0xBD, 0x81, 0x29, 0x77, 0xA2, 0x0F, 0x45, 0x8E, 0x5A, 0x64, 0x18, 0xCF, 0x37},
    .nvs_key = NULL,
    .prop = ESP_GATT_CHAR_PROP_BIT_WRITE,
    .max_length = 16};

#define NAOS_BLE_NUM_CHARS 12

static naos_ble_gatts_char_t *naos_ble_gatts_chars[NAOS_BLE_NUM_CHARS] = {
    &naos_ble_char_wifi_ssid,     &naos_ble_char_wifi_password,     &naos_ble_char_mqtt_host,
    &naos_ble_char_mqtt_port,     &naos_ble_char_mqtt_client_id,    &naos_ble_char_mqtt_username,
    &naos_ble_char_mqtt_password, &naos_ble_char_device_type,       &naos_ble_char_device_name,
    &naos_ble_char_base_topic,    &naos_ble_char_connection_status, &naos_ble_char_system_command};

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
      // ESP_LOGI(NAOS_LOG_TAG, "Unhandled GAP Event: %d", e);
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

        // move on if uuids do not match
        if (memcmp(p->add_char.char_uuid.uuid.uuid128, c->_uuid.uuid.uuid128, ESP_UUID_LEN_128) != 0) {
          continue;
        }

        // save attribute handle
        c->handle = p->add_char.attr_handle;

        // set initialization bit if this is the last characteristic
        if (j + 1 == NAOS_BLE_NUM_CHARS) {
          xEventGroupSetBits(naos_ble_init_event_group, NAOS_BLE_INITIALIZED_BIT);
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
      // save connection handle
      naos_ble_gatts_profile.client_connected = true;
      naos_ble_gatts_profile.client_handle = p->connect.conn_id;

      break;
    }

    // handle characteristic read event
    case ESP_GATTS_READ_EVT: {
      // break immediately if no response is needed
      if (!p->read.need_rsp) {
        break;
      }

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
        }

        // prepare response
        esp_gatt_rsp_t rsp;
        memset(&rsp, 0, sizeof(esp_gatt_rsp_t));

        // set handle
        rsp.attr_value.handle = c->handle;

        // set data
        strcpy((char *)rsp.attr_value.value, (char *)c->storage);
        rsp.attr_value.len = c->length;

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
        }

        // check attribute length and return if it exceeds
        if (p->write.len > c->max_length) {
          // send error response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_INVALID_ATTR_LEN, NULL));
        }

        // save attribute
        memcpy((char *)c->storage, (char *)p->write.value, p->write.len);
        c->storage[p->write.len] = '\0';

        // set current length
        c->length = (uint8_t)p->write.len;

        // save attribute to flash if key is present
        if (c->nvs_key != NULL) {
          ESP_ERROR_CHECK(nvs_set_str(naos_ble_nvs_handle, c->nvs_key, (char *)c->storage));
          ESP_ERROR_CHECK(nvs_commit(naos_ble_nvs_handle));
        }

        // send response if requested
        if (p->write.need_rsp) {
          ESP_ERROR_CHECK(esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_OK, NULL));
        }

        // call callback if available
        if (naos_ble_callback != NULL) {
          NAOS_UNLOCK(naos_ble_mutex);
          naos_ble_callback(c->id);
          NAOS_LOCK(naos_ble_mutex);
        }

        // exit loop
        break;
      }

      break;
    }

    // handle client disconnect event
    case ESP_GATTS_DISCONNECT_EVT: {
      // reset connection handle
      naos_ble_gatts_profile.client_connected = false;
      naos_ble_gatts_profile.client_handle = 0;

      // restart advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    default: {
      // ESP_LOGI(NAOS_LOG_TAG, "Unhandled GATTS Event: %d", e);
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_ble_mutex);
}

void naos_ble_init(naos_ble_attribute_callback_t cb, const char *device_type) {
  // create mutex
  naos_ble_mutex = xSemaphoreCreateMutex();

  // set callback
  naos_ble_callback = cb;

  // create even group
  naos_ble_init_event_group = xEventGroupCreate();

  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-ble", NVS_READWRITE, &naos_ble_nvs_handle));

  // iterate through all characteristics
  for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
    // get pointer of current characteristic
    naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

    // setup uuid
    c->_uuid.len = ESP_UUID_LEN_128;
    memcpy(c->_uuid.uuid.uuid128, c->uuid, ESP_UUID_LEN_128);

    // allocate storage
    c->storage = malloc(c->max_length + 1);

    // move on if no nvs key is set
    if (c->nvs_key == NULL) {
      continue;
    }

    // prepare length
    size_t length = c->max_length + 1;

    // read value
    esp_err_t err = nvs_get_str(naos_ble_nvs_handle, c->nvs_key, (char *)c->storage, &length);
    if (err != ESP_ERR_NVS_NOT_FOUND) {
      ESP_ERROR_CHECK(err);
    }

    // save length
    c->length = (uint8_t)(err == ESP_ERR_NVS_NOT_FOUND ? 0 : strlen((char *)c->storage));

    // null terminate value
    c->storage[c->length] = '\0';
  }

  // set device type
  strcpy((char *)naos_ble_char_device_type.storage, device_type);
  naos_ble_char_device_type.length = (uint8_t)strlen(device_type);

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
  xEventGroupWaitBits(naos_ble_init_event_group, NAOS_BLE_INITIALIZED_BIT, pdFALSE, pdFALSE, portMAX_DELAY);
}

char *naos_ble_get_string(naos_ble_id_t id) {
  // acquire mutex
  NAOS_LOCK(naos_ble_mutex);

  // iterate through all characteristics
  for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
    // get pointer of current characteristic
    naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

    // check if ids match
    if (c->id == id) {
      // duplicate string
      char *str = strdup((const char *)c->storage);

      // release mutex
      NAOS_UNLOCK(naos_ble_mutex);

      return str;
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_ble_mutex);

  return strdup("");
}

void naos_ble_set_string(naos_ble_id_t id, const char *str) {
  // acquire mutex
  NAOS_LOCK(naos_ble_mutex);

  // calculate length
  size_t len = strlen(str);

  // iterate through all characteristics
  for (int j = 0; j < NAOS_BLE_NUM_CHARS; j++) {
    // get pointer of current characteristic
    naos_ble_gatts_char_t *c = naos_ble_gatts_chars[j];

    // check if id matches and size is below the defined max length
    if (c->id == id && len <= c->max_length) {
      // copy string
      strcpy((char *)c->storage, str);

      // set length
      c->length = (uint8_t)len;

      // send indicate if indicate is supported and client is connected
      if (c->prop & ESP_GATT_CHAR_PROP_BIT_INDICATE && naos_ble_gatts_profile.client_connected) {
        ESP_ERROR_CHECK(esp_ble_gatts_send_indicate(naos_ble_gatts_profile.interface,
                                                    naos_ble_gatts_profile.client_handle, c->handle, c->length,
                                                    c->storage, false));
      }

      // exit loop
      break;
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_ble_mutex);
}
