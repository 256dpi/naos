#include <naos/auth.h>
#include <naos/msg.h>
#include <esp_efuse.h>
#include <esp_efuse_table.h>
#include <esp_hmac.h>
#include <string.h>

#ifdef CONFIG_EFUSE_VIRTUAL
#include <mbedtls/md.h>
#endif  // CONFIG_EFUSE_VIRTUAL

#define NAOS_AUTH_KEY HMAC_KEY5
#define NAOS_AUTH_KEY_BLOCK EFUSE_BLK_KEY5
#define NAOS_AUTH_DATA_BLOCK EFUSE_BLK_USER_DATA
#define NAOS_AUTH_ENDPOINT 0x6

static_assert(sizeof(naos_auth_data_t) == 32, "naos_auth_data_t must be 32 bytes");

typedef enum {
  NAOS_AUTH_CMD_STATUS,
  NAOS_AUTH_CMD_PROVISION,
  NAOS_AUTH_CMD_DESCRIBE,
  NAOS_AUTH_CMD_ATTEST,
} naos_auth_cmd_t;

#ifdef CONFIG_EFUSE_VIRTUAL
static uint8_t naos_auth_key_block[32] = {0};
static bool naos_auth_key_written = false;
#endif  // CONFIG_EFUSE_VIRTUAL

static esp_err_t naos_auth_hmac(const void *message, size_t message_len, uint8_t *hmac) {
  // calculate HMAC in software, if efuse is virtual
#ifdef CONFIG_EFUSE_VIRTUAL
  const mbedtls_md_info_t *info = mbedtls_md_info_from_type(MBEDTLS_MD_SHA256);
  int ret = mbedtls_md_hmac(info, naos_auth_key_block, 32, message, message_len, hmac);
  if (ret != 0) {
    return ESP_ERR_INVALID_ARG;
  }
  return ESP_OK;
#endif

  // otherwise, calculate HMAC using hardware
  esp_err_t err = esp_hmac_calculate(NAOS_AUTH_KEY, message, message_len, hmac);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static naos_msg_reply_t naos_auth_handle(naos_msg_t msg) {
  // check length
  if (msg.len < 1) {
    return NAOS_MSG_INVALID;
  }

  // pluck of command
  naos_auth_cmd_t cmd = (naos_auth_cmd_t)msg.data[0];
  msg.data++;
  msg.len--;

  // handle command
  switch (cmd) {
    case NAOS_AUTH_CMD_STATUS: {
      // check length
      if (msg.len != 0) {
        return NAOS_MSG_INVALID;
      }

      // check status
      uint8_t status = naos_auth_status() ? 1 : 0;
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = msg.endpoint,
          .data = &status,
          .len = sizeof(status),
      });

      return NAOS_MSG_OK;
    }

    case NAOS_AUTH_CMD_PROVISION: {
      // check length
      if (msg.len != 64) {
        return NAOS_MSG_INVALID;
      }

      // provision key and data
      naos_auth_data_t *data = (naos_auth_data_t *)(msg.data + 32);
      naos_auth_err_t err = naos_auth_provision(msg.data, data);
      if (err != NAOS_AUTH_ERR_OK) {
        return NAOS_MSG_ERROR;
      }

      return NAOS_MSG_ACK;
    }

    case NAOS_AUTH_CMD_DESCRIBE: {
      // check length
      if (msg.len != 0) {
        return NAOS_MSG_INVALID;
      }

      // describe data
      naos_auth_data_t data;
      naos_auth_err_t err = naos_auth_describe(&data);
      if (err != NAOS_AUTH_ERR_OK) {
        return NAOS_MSG_ERROR;
      }

      // send data
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = msg.endpoint,
          .data = (void *)&data,
          .len = sizeof(naos_auth_data_t),
      });

      return NAOS_MSG_OK;
    }

    case NAOS_AUTH_CMD_ATTEST: {
      // authenticate challenge
      uint8_t response[32];
      naos_auth_err_t err = naos_auth_attest(msg.data, msg.len, response);
      if (err != NAOS_AUTH_ERR_OK) {
        return NAOS_MSG_ERROR;
      }

      // write response
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = msg.endpoint,
          .data = response,
          .len = sizeof(response),
      });

      return NAOS_MSG_OK;
    }

    default:
      return NAOS_MSG_UNKNOWN;
  }
}

void naos_auth_install() {
  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_AUTH_ENDPOINT,
      .name = "auth",
      .handle = naos_auth_handle,
  });
}

bool naos_auth_status() {
  // check protection bits
#ifdef CONFIG_EFUSE_VIRTUAL
  bool key = naos_auth_key_written;
#else
  bool key = esp_efuse_read_field_bit(ESP_EFUSE_WR_DIS_KEY5) && esp_efuse_read_field_bit(ESP_EFUSE_RD_DIS_KEY5);
#endif
  bool data = esp_efuse_read_field_bit(ESP_EFUSE_WR_DIS_USER_DATA);

  return key && data;
}

naos_auth_err_t naos_auth_provision(uint8_t key[32], naos_auth_data_t *data) {
  // check status
  if (naos_auth_status()) {
    return NAOS_AUTH_ERR_ALREADY_PROVISIONED;
  }

  // check version
  if (data->version != 1) {
    return NAOS_AUTH_ERR_INVALID_VERSION;
  }

  // write key
#ifdef CONFIG_EFUSE_VIRTUAL
  if (naos_auth_key_written) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }
  memcpy(naos_auth_key_block, key, 32);
  naos_auth_key_written = true;
#else
  ESP_ERROR_CHECK(esp_efuse_write_key(NAOS_AUTH_KEY_BLOCK, ESP_EFUSE_KEY_PURPOSE_HMAC_UP, key, 32));
#endif

  // calculate signature
  uint8_t signature[32];
  ESP_ERROR_CHECK(naos_auth_hmac(data, sizeof(naos_auth_data_t) - sizeof(data->signature), signature));

  // set signature
  memcpy(data->signature, signature, sizeof(data->signature));

  // write and protect block
  ESP_ERROR_CHECK(esp_efuse_batch_write_begin());
  ESP_ERROR_CHECK(esp_efuse_write_block(NAOS_AUTH_DATA_BLOCK, (uint8_t *)data, 0, sizeof(naos_auth_data_t) * 8));
  ESP_ERROR_CHECK(esp_efuse_set_write_protect(NAOS_AUTH_DATA_BLOCK));
  ESP_ERROR_CHECK(esp_efuse_batch_write_commit());

  return NAOS_AUTH_ERR_OK;
}

naos_auth_err_t naos_auth_describe(naos_auth_data_t *data) {
  // check status
  if (!naos_auth_status()) {
    return NAOS_AUTH_ERR_NOT_PROVISIONED;
  }

  // read block
  ESP_ERROR_CHECK(esp_efuse_read_block(NAOS_AUTH_DATA_BLOCK, (uint8_t *)data, 0, sizeof(naos_auth_data_t) * 8));

  // check version
  if (data->version != 1) {
    return NAOS_AUTH_ERR_INVALID_VERSION;
  }

  // re-calculate signature
  uint8_t signature[32];
  ESP_ERROR_CHECK(naos_auth_hmac(data, sizeof(naos_auth_data_t) - sizeof(data->signature), signature));

  // verify signature
  if (memcmp(signature, data->signature, sizeof(data->signature)) != 0) {
    return NAOS_AUTH_ERR_INVALID_SIGNATURE;
  }

  return NAOS_AUTH_ERR_OK;
}

naos_auth_err_t naos_auth_attest(const void *challenge, size_t length, uint8_t response[32]) {
  // check status
  if (!naos_auth_status()) {
    return NAOS_AUTH_ERR_NOT_PROVISIONED;
  }

  // calculate HMAC
  ESP_ERROR_CHECK(naos_auth_hmac(challenge, length, response));

  return NAOS_AUTH_ERR_OK;
}
