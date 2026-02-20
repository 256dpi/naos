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
#include <nvs.h>
#include <string.h>

#include "params.h"
#include "utils.h"

// TODO: naos_ble_pending_id is shared state without protection; concurrent bonding could overwrite a pending identity.
// TODO: allowlist ring buffer doesn't remove evicted entries from the controller whitelist/resolving list.

#define NAOS_BLE_SIGNAL_INIT (1 << 0)
#define NAOS_BLE_SIGNAL_CONN (1 << 1)
#define NAOS_BLE_SIGNAL_ADV (1 << 2)
#define NAOS_BLE_ALLOWLIST_SIZE 5
#define NAOS_BLE_ALLOWLIST_KEY "allowlist"
#define NAOS_BLE_NUM_CHARS 1
#define NAOS_BLE_MAX_CONNECTIONS 8
#define NAOS_BLE_ADDR_FMT "%02x:%02x:%02x:%02x:%02x:%02x"
#define NAOS_BLE_ADDR_ARGS(a) a[0], a[1], a[2], a[3], a[4], a[5]
#define NAOS_BLE_WL_ADDR_TYPE(t) ((t) == BLE_ADDR_TYPE_PUBLIC ? BLE_WL_ADDR_TYPE_PUBLIC : BLE_WL_ADDR_TYPE_RANDOM)

typedef struct {
  uint16_t id;
  uint16_t mtu;
  bool congested;
  bool connected;
} naos_ble_conn_t;

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
  // ---
  uint16_t handle;
  esp_bt_uuid_t _uuid;
} naos_ble_gatts_char_t;

static naos_ble_gatts_char_t naos_ble_char_msg = {
    .uuid = {0xf3, 0x30, 0x41, 0x63, 0xf3, 0x37, 0x45, 0xc9, 0xad, 0x00, 0x1b, 0xa6, 0x4b, 0x74, 0x60, 0x03},
    .prop = ESP_GATT_CHAR_PROP_BIT_WRITE | ESP_GATT_CHAR_PROP_BIT_WRITE_NR | ESP_GATT_CHAR_PROP_BIT_INDICATE,
};

static naos_ble_gatts_char_t *naos_ble_gatts_chars[NAOS_BLE_NUM_CHARS] = {
    &naos_ble_char_msg,
};

static struct {
  struct {
    esp_bd_addr_t addr;
    esp_ble_addr_type_t type;
    uint8_t irk[ESP_BT_OCTET16_LEN];
    bool has_irk;
  } entries[NAOS_BLE_ALLOWLIST_SIZE];
  size_t next;
} naos_ble_allowlist = {0};

static struct {
  esp_bd_addr_t addr;
  esp_ble_addr_type_t type;
  uint8_t irk[ESP_BT_OCTET16_LEN];
  bool valid;
} naos_ble_pending_id = {0};

static naos_ble_config_t naos_ble_config = {0};
static naos_signal_t naos_ble_signal;
static nvs_handle_t naos_ble_handle = 0;
static naos_ble_conn_t naos_ble_conns[NAOS_BLE_MAX_CONNECTIONS];
static uint8_t naos_ble_msg_channel_id = 0;
static bool naos_ble_stop_adv_for_rl = false;  // set when we stop advertising to update resolving list

static void naos_ble_gap_handler(esp_gap_ble_cb_event_t e, esp_ble_gap_cb_param_t *p) {
  switch (e) {
    case ESP_GAP_BLE_ADV_DATA_SET_COMPLETE_EVT: {
      // trigger signal
      naos_trigger(naos_ble_signal, NAOS_BLE_SIGNAL_ADV, false);

      break;
    }

    case ESP_GAP_BLE_ADV_START_COMPLETE_EVT: {
      // check status
      if (p->adv_start_cmpl.status != ESP_BT_STATUS_SUCCESS) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_ble_gap_handler: failed to start advertisement (%d)", p->adv_start_cmpl.status);
        break;
      }

      break;
    }

    case ESP_GAP_BLE_UPDATE_CONN_PARAMS_EVT: {
      // check status
      if (p->update_conn_params.status != ESP_BT_STATUS_SUCCESS) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_ble_gap_handler: failed to update connection parameters (%d)",
                 p->update_conn_params.status);
        break;
      }

      // log info
      ESP_LOGI(NAOS_LOG_TAG,
               "naos_ble_gap_handler: connection parameters updated (min_int=%d max_int=%d latency=%d conn_int=%d "
               "timeout=%d)",
               p->update_conn_params.min_int, p->update_conn_params.max_int, p->update_conn_params.latency,
               p->update_conn_params.conn_int, p->update_conn_params.timeout);

      break;
    }

    case ESP_GAP_BLE_SEC_REQ_EVT: {
      // auto accept security requests
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gap_handler: security request from peer, accepting...");
      ESP_ERROR_CHECK(esp_ble_gap_security_rsp(p->ble_security.ble_req.bd_addr, true));

      break;
    }

    case ESP_GAP_BLE_PASSKEY_REQ_EVT: {
      // auto accept passkey requests
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gap_handler: passkey request (Just Works) - auto accepting");

      break;
    }

    case ESP_GAP_BLE_KEY_EVT: {
      // log key events
      ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gap_handler: key event (type=%d)", p->ble_security.ble_key.key_type);

      // capture peer identity key (stable identity address + IRK behind the RPA)
      if (p->ble_security.ble_key.key_type == ESP_LE_KEY_PID) {
        esp_ble_pid_keys_t *pid = &p->ble_security.ble_key.p_key_value.pid_key;
        memcpy(naos_ble_pending_id.addr, pid->static_addr, sizeof(esp_bd_addr_t));
        naos_ble_pending_id.type = pid->addr_type;
        memcpy(naos_ble_pending_id.irk, pid->irk, ESP_BT_OCTET16_LEN);
        naos_ble_pending_id.valid = true;
        ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gap_handler: peer identity (type=%d addr=" NAOS_BLE_ADDR_FMT ")",
                 naos_ble_pending_id.type, NAOS_BLE_ADDR_ARGS(naos_ble_pending_id.addr));
      }

      break;
    }

    case ESP_GAP_BLE_AUTH_CMPL_EVT: {
      // log authentication completion
      if (p->ble_security.auth_cmpl.success) {
        ESP_LOGI(NAOS_LOG_TAG, "naos_ble_gap_handler: authentication complete (mode=%d)",
                 p->ble_security.auth_cmpl.auth_mode);
      } else {
        ESP_LOGW(NAOS_LOG_TAG, "naos_ble_gap_handler: authentication failed");
        break;
      }

      // commit identity address + IRK to whitelist and NVS (resolving list add is deferred to ADV_STOP_COMPLETE)
      if (naos_ble_config.pairing && naos_ble_pending_id.valid) {
        // add to controller whitelist
        ESP_ERROR_CHECK(esp_ble_gap_update_whitelist(true, naos_ble_pending_id.addr,
                                                     NAOS_BLE_WL_ADDR_TYPE(naos_ble_pending_id.type)));

        // persist identity in allowlist if not already present
        bool found = false;
        for (size_t j = 0; j < NAOS_BLE_ALLOWLIST_SIZE; j++) {
          if (memcmp(naos_ble_allowlist.entries[j].addr, naos_ble_pending_id.addr, sizeof(esp_bd_addr_t)) == 0) {
            found = true;
            break;
          }
        }
        if (!found) {
          size_t idx = naos_ble_allowlist.next;
          memcpy(naos_ble_allowlist.entries[idx].addr, naos_ble_pending_id.addr, sizeof(esp_bd_addr_t));
          naos_ble_allowlist.entries[idx].type = naos_ble_pending_id.type;
          memcpy(naos_ble_allowlist.entries[idx].irk, naos_ble_pending_id.irk, ESP_BT_OCTET16_LEN);
          naos_ble_allowlist.entries[idx].has_irk = true;
          naos_ble_allowlist.next = (naos_ble_allowlist.next + 1) % NAOS_BLE_ALLOWLIST_SIZE;
          ESP_ERROR_CHECK(
              nvs_set_blob(naos_ble_handle, NAOS_BLE_ALLOWLIST_KEY, &naos_ble_allowlist, sizeof(naos_ble_allowlist)));
          ESP_ERROR_CHECK(nvs_commit(naos_ble_handle));
          ESP_LOGI(NAOS_LOG_TAG,
                   "naos_ble_gap_handler: added identity to allowlist (type=%d addr=" NAOS_BLE_ADDR_FMT ")",
                   naos_ble_pending_id.type, NAOS_BLE_ADDR_ARGS(naos_ble_pending_id.addr));
        }

        // stop advertising so ADV_STOP_COMPLETE can safely add to resolving list
        esp_err_t err = esp_ble_gap_stop_advertising();
        if (err == ESP_OK) {
          naos_ble_stop_adv_for_rl = true;
        } else if (err == ESP_ERR_INVALID_STATE) {  // not advertising
          ESP_ERROR_CHECK(esp_ble_gap_add_device_to_resolving_list(naos_ble_pending_id.addr, naos_ble_pending_id.type,
                                                                   naos_ble_pending_id.irk));
          naos_ble_pending_id.valid = false;
          ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));
        } else {
          ESP_LOGW(NAOS_LOG_TAG, "naos_ble_gap_handler: failed to stop advertising, dropping pending identity (%d)",
                   err);
          naos_ble_pending_id.valid = false;
        }
      }

      break;
    }

    case ESP_GAP_BLE_ADV_STOP_COMPLETE_EVT: {
      // only handle if we deliberately stopped advertising for a resolving list update
      if (!naos_ble_stop_adv_for_rl) {
        break;
      }

      // clear flag
      naos_ble_stop_adv_for_rl = false;

      // check status
      if (p->adv_stop_cmpl.status != ESP_BT_STATUS_SUCCESS) {
        ESP_LOGW(NAOS_LOG_TAG, "naos_ble_gap_handler: adv stop failed during RL update (%d)", p->adv_stop_cmpl.status);
        naos_ble_pending_id.valid = false;
        break;
      }

      // add to resolving list now that advertising is stopped (required by controller)
      if (naos_ble_pending_id.valid) {
        ESP_ERROR_CHECK(esp_ble_gap_add_device_to_resolving_list(naos_ble_pending_id.addr, naos_ble_pending_id.type,
                                                                 naos_ble_pending_id.irk));
        naos_ble_pending_id.valid = false;
      }

      // restart advertising
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      break;
    }

    default: {
      ESP_LOGD(NAOS_LOG_TAG, "naos_ble_gap_handler: unhandled event: %d", e);
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
        if (naos_ble_config.bonding) {
          perm = ESP_GATT_PERM_READ_ENCRYPTED | ESP_GATT_PERM_WRITE_ENCRYPTED;
        }

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
          naos_trigger(naos_ble_signal, NAOS_BLE_SIGNAL_INIT, false);
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

      // trigger signal
      naos_trigger(naos_ble_signal, NAOS_BLE_SIGNAL_CONN, false);

      // in pairing-only mode (no bonding), add the device address directly to the allowlist on connect,
      // since there is no AUTH_CMPL to derive a stable identity from
      if (naos_ble_config.pairing && !naos_ble_config.bonding) {
        ESP_LOGI(NAOS_LOG_TAG,
                 "naos_ble_gatts_handler: adding address to allowlist (type=%d addr=" NAOS_BLE_ADDR_FMT ")",
                 p->connect.ble_addr_type, NAOS_BLE_ADDR_ARGS(p->connect.remote_bda));
        ESP_ERROR_CHECK(
            esp_ble_gap_update_whitelist(true, p->connect.remote_bda, NAOS_BLE_WL_ADDR_TYPE(p->connect.ble_addr_type)));
        bool found = false;
        for (size_t j = 0; j < NAOS_BLE_ALLOWLIST_SIZE; j++) {
          if (memcmp(naos_ble_allowlist.entries[j].addr, p->connect.remote_bda, sizeof(esp_bd_addr_t)) == 0) {
            found = true;
            break;
          }
        }
        if (!found) {
          size_t idx = naos_ble_allowlist.next;
          memcpy(naos_ble_allowlist.entries[idx].addr, p->connect.remote_bda, sizeof(esp_bd_addr_t));
          naos_ble_allowlist.entries[idx].type = p->connect.ble_addr_type;
          naos_ble_allowlist.next = (naos_ble_allowlist.next + 1) % NAOS_BLE_ALLOWLIST_SIZE;
          ESP_ERROR_CHECK(
              nvs_set_blob(naos_ble_handle, NAOS_BLE_ALLOWLIST_KEY, &naos_ble_allowlist, sizeof(naos_ble_allowlist)));
          ESP_ERROR_CHECK(nvs_commit(naos_ble_handle));
        }
      }

      // restart advertisement
      ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));

      // start encryption immediately if bonding is enabled
      if (naos_ble_config.bonding) {
        ESP_ERROR_CHECK(esp_ble_set_encryption(p->connect.remote_bda, ESP_BLE_SEC_ENCRYPT_NO_MITM));
      }

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

        // unused, but future readable characteristics would be handled here

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
        if (p->write.offset + p->write.len > ESP_GATT_MAX_MTU_SIZE) {
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_INVALID_ATTR_LEN, NULL));
          return;
        }

        // prepare status
        esp_gatt_status_t status = ESP_GATT_OK;

        // handle write prepare
        if (p->write.is_prep) {
          // Note: Supported in #8ba40f0, but removed again due to memory overhead.
          // Instead, make sure that message is below MTU and sent as a normal write.

          // log error
          ESP_LOGE(NAOS_LOG_TAG, "naos_ble_gatts_handler: unsupported long write (id=%d, len=%d)", p->write.conn_id,
                   p->write.len);

          // send response
          ESP_ERROR_CHECK(
              esp_ble_gatts_send_response(i, p->write.conn_id, p->write.trans_id, ESP_GATT_REQ_NOT_SUPPORTED, NULL));

          return;
        }

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

    // handle execute write event
    case ESP_GATTS_EXEC_WRITE_EVT: {
      ESP_ERROR_CHECK(
          esp_ble_gatts_send_response(i, p->exec_write.conn_id, p->exec_write.trans_id, ESP_GATT_REQ_NOT_SUPPORTED, NULL));

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
      ESP_LOGD(NAOS_LOG_TAG, "naos_ble_gatts_handler: unhandled event: %d", e);
    }
  }
}

static void naos_ble_set_name() {
  // prepare name
  const char *name = naos_get_s("device-name");

  // use app name if absent
  if (strlen(name) == 0) {
    name = naos_config()->app_name;
  }

  // cap name to not exceed adv packet
  char copy[9] = {0};
  if (strlen(name) > 8) {
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

  // calculate MTU
  uint16_t mtu = conn->mtu - 5;  // 5 bytes are reserved for the BLE stack;

  return mtu;
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
      ESP_LOGW(NAOS_LOG_TAG, "naos_ble_msg_send: failed to send msg as notification (%d)", err);
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

  // adjust the advertisement filter policy
  if (cfg.pairing) {
    naos_ble_adv_params.adv_filter_policy = ADV_FILTER_ALLOW_SCAN_WLST_CON_WLST;
  }

  // enable local privacy so the controller uses the resolving list for RPA resolution
  if (cfg.pairing && cfg.bonding) {
    naos_ble_adv_params.own_addr_type = BLE_ADDR_TYPE_RPA_PUBLIC;
    ESP_ERROR_CHECK(esp_ble_gap_config_local_privacy(true));
  }

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

  // setup encryption if bonding is enabled
  if (cfg.bonding) {
    esp_ble_auth_req_t auth_req = ESP_LE_AUTH_BOND;
    esp_ble_io_cap_t io_cap = ESP_IO_CAP_NONE;
    uint8_t key_size = 16;
    uint8_t init_key = ESP_BLE_ENC_KEY_MASK | ESP_BLE_ID_KEY_MASK;
    uint8_t resp_key = ESP_BLE_ENC_KEY_MASK | ESP_BLE_ID_KEY_MASK;
    ESP_ERROR_CHECK(esp_ble_gap_set_security_param(ESP_BLE_SM_AUTHEN_REQ_MODE, &auth_req, sizeof(auth_req)));
    ESP_ERROR_CHECK(esp_ble_gap_set_security_param(ESP_BLE_SM_IOCAP_MODE, &io_cap, sizeof(io_cap)));
    ESP_ERROR_CHECK(esp_ble_gap_set_security_param(ESP_BLE_SM_MAX_KEY_SIZE, &key_size, sizeof(key_size)));
    ESP_ERROR_CHECK(esp_ble_gap_set_security_param(ESP_BLE_SM_SET_INIT_KEY, &init_key, sizeof(init_key)));
    ESP_ERROR_CHECK(esp_ble_gap_set_security_param(ESP_BLE_SM_SET_RSP_KEY, &resp_key, sizeof(resp_key)));
  }

  // register application
  ESP_ERROR_CHECK(esp_ble_gatts_app_register(0x55));
  naos_await(naos_ble_signal, NAOS_BLE_SIGNAL_INIT, true, -1);

  // open nvs namespace
  ESP_ERROR_CHECK(nvs_open("naos-ble", NVS_READWRITE, &naos_ble_handle));

  // restore allowlist before advertising starts (resolving list add requires advertising to be stopped)
  if (naos_ble_config.pairing) {
    size_t size = 0;
    esp_err_t err = nvs_get_blob(naos_ble_handle, NAOS_BLE_ALLOWLIST_KEY, NULL, &size);
    if (err != ESP_OK && err != ESP_ERR_NVS_NOT_FOUND) {
      ESP_ERROR_CHECK(err);
    }
    if (size == sizeof(naos_ble_allowlist)) {
      ESP_ERROR_CHECK(nvs_get_blob(naos_ble_handle, NAOS_BLE_ALLOWLIST_KEY, &naos_ble_allowlist, &size));
      for (size_t i = 0; i < NAOS_BLE_ALLOWLIST_SIZE; i++) {
        esp_bd_addr_t zero = {0};
        if (memcmp(naos_ble_allowlist.entries[i].addr, zero, sizeof(esp_bd_addr_t)) != 0) {
          esp_ble_wl_addr_type_t wl_type = NAOS_BLE_WL_ADDR_TYPE(naos_ble_allowlist.entries[i].type);
          ESP_LOGI(NAOS_LOG_TAG, "naos_ble_init: restoring allowlist entry (type=%d addr=" NAOS_BLE_ADDR_FMT ")",
                   naos_ble_allowlist.entries[i].type, NAOS_BLE_ADDR_ARGS(naos_ble_allowlist.entries[i].addr));
          ESP_ERROR_CHECK(esp_ble_gap_update_whitelist(true, naos_ble_allowlist.entries[i].addr, wl_type));
          if (naos_ble_allowlist.entries[i].has_irk) {
            ESP_ERROR_CHECK(esp_ble_gap_add_device_to_resolving_list(naos_ble_allowlist.entries[i].addr,
                                                                     naos_ble_allowlist.entries[i].type,
                                                                     naos_ble_allowlist.entries[i].irk));
          }
        }
      }
    }
  }

  // handle parameters
  naos_params_subscribe(naos_ble_param_handler);

  // register channel
  naos_ble_msg_channel_id = naos_msg_register((naos_msg_channel_t){
      .name = "ble",
      .mtu = naos_ble_msg_mtu,
      .send = naos_ble_msg_send,
  });

  // clear signal
  naos_trigger(naos_ble_signal, NAOS_BLE_SIGNAL_ADV, true);

  // set device name
  naos_ble_set_name();

  // await signal
  naos_await(naos_ble_signal, NAOS_BLE_SIGNAL_ADV, true, -1);

  // start advertising
  ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));
}

bool naos_ble_await(int32_t timeout_ms) {
  // clear signal
  naos_trigger(naos_ble_signal, NAOS_BLE_SIGNAL_CONN, true);

  // await connection
  return naos_await(naos_ble_signal, NAOS_BLE_SIGNAL_CONN, true, timeout_ms);
}

int naos_ble_connections() {
  // count connections
  int count = 0;
  for (int i = 0; i < NAOS_BLE_MAX_CONNECTIONS; i++) {
    if (naos_ble_conns[i].connected) {
      count++;
    }
  }

  return count;
}

void naos_ble_enable_pairing() {
  // check if pairing is enabled
  if (!naos_ble_config.pairing) {
    ESP_ERROR_CHECK(ESP_FAIL);
    return;
  }

  // set open advertisement filter policy
  naos_ble_adv_params.adv_filter_policy = ADV_FILTER_ALLOW_SCAN_ANY_CON_ANY;
  ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));
}

void naos_ble_disable_pairing() {
  // check if pairing is enabled
  if (!naos_ble_config.pairing) {
    ESP_ERROR_CHECK(ESP_FAIL);
    return;
  }

  // set closed advertisement filter policy
  naos_ble_adv_params.adv_filter_policy = ADV_FILTER_ALLOW_SCAN_WLST_CON_WLST;
  ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));
}

int naos_ble_allowlist_length() {
  // count allowlist entries
  int count = 0;
  for (size_t i = 0; i < NAOS_BLE_ALLOWLIST_SIZE; i++) {
    esp_bd_addr_t zero = {0};
    if (memcmp(naos_ble_allowlist.entries[i].addr, zero, sizeof(esp_bd_addr_t)) != 0) {
      count++;
    }
  }

  return count;
}

void naos_ble_allowlist_clear() {
  // stop if empty
  if (naos_ble_allowlist_length() == 0) {
    return;
  }

  // clear allowlist
  memset(&naos_ble_allowlist, 0, sizeof(naos_ble_allowlist));
  ESP_ERROR_CHECK(
      nvs_set_blob(naos_ble_handle, NAOS_BLE_ALLOWLIST_KEY, &naos_ble_allowlist, sizeof(naos_ble_allowlist)));
  ESP_ERROR_CHECK(nvs_commit(naos_ble_handle));
  ESP_ERROR_CHECK(esp_ble_gap_clear_whitelist());

  // clear resolving list by toggling local privacy off and on
  if (naos_ble_config.bonding) {
    esp_ble_gap_stop_advertising();
    ESP_ERROR_CHECK(esp_ble_gap_config_local_privacy(false));
    ESP_ERROR_CHECK(esp_ble_gap_config_local_privacy(true));
    ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));
  }
}

int naos_ble_peerlist_length() {
  // return the number of bonded devices
  return esp_ble_get_bond_device_num();
}

void naos_ble_peerlist_clear() {
  // enumerate bonded devices
  int num = esp_ble_get_bond_device_num();
  if (num == 0) {
    return;
  }

  // allocate device list
  esp_ble_bond_dev_t *list = malloc(num * sizeof(esp_ble_bond_dev_t));
  if (!list) {
    ESP_ERROR_CHECK(ESP_FAIL);
    return;
  }

  // fill device list
  ESP_ERROR_CHECK(esp_ble_get_bond_device_list(&num, list));

  // remove devices
  for (int i = 0; i < num; i++) {
    ESP_ERROR_CHECK(esp_ble_remove_bond_device(list[i].bd_addr));
    ESP_LOGI(NAOS_LOG_TAG, "naos_ble_peerlist_clear: removed bonded device (addr=" NAOS_BLE_ADDR_FMT ")",
             NAOS_BLE_ADDR_ARGS(list[i].bd_addr));
  }

  // free list
  free(list);

  // clear resolving list by toggling local privacy off and on
  if (naos_ble_config.bonding) {
    esp_ble_gap_stop_advertising();
    ESP_ERROR_CHECK(esp_ble_gap_config_local_privacy(false));
    ESP_ERROR_CHECK(esp_ble_gap_config_local_privacy(true));
    ESP_ERROR_CHECK(esp_ble_gap_start_advertising(&naos_ble_adv_params));
  }
}
