#ifndef NAOS_AUTH_H
#define NAOS_AUTH_H

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

// Note: Enable CONFIG_EFUSE_VIRTUAL in menuconfig to use virtual eFuses for
// testing. The virtual eFuses are not persistent and will reset on reboot. The
// authentication key is also stored in memory and reset on boot.

/**
 * Data structure containing the authentication data for the device.
 */
typedef struct __attribute__((packed)) {
  uint8_t version;  // 1
  uint8_t uuid[16];
  uint16_t product;
  uint16_t revision;
  uint16_t batch;
  uint32_t date;  // epoch seconds
  uint8_t signature[5];
} naos_auth_data_t;

typedef enum {
  NAOS_AUTH_ERR_OK = 0,
  NAOS_AUTH_ERR_INVALID_VERSION,
  NAOS_AUTH_ERR_ALREADY_PROVISIONED,
  NAOS_AUTH_ERR_NOT_PROVISIONED,
  NAOS_AUTH_ERR_INVALID_SIGNATURE,
} naos_auth_err_t;

/**
 * Install the authentication endpoint.
 */
void naos_auth_install();

/**
 * Returns whether the device is provisioned.
 */
bool naos_auth_status();

/**
 * Provision the device with a key and data.
 *
 * @param key The HMAC authentication key.
 * @param data The supplemental device data.
 * @return
 *  - NAOS_AUTH_ERR_OK: Successful provisioning.
 *  - NAOS_AUTH_ERR_INVALID_VERSION: If the version in data is not 1.
 *  - NAOS_AUTH_ERR_ALREADY_PROVISIONED: If the device is already provisioned.
 */
naos_auth_err_t naos_auth_provision(uint8_t key[32], naos_auth_data_t *data);

/**
 * Describe the device by reading the provisioned data.
 *
 * @param data Pointer to structure to fill with device data.
 * @return
 *  - NAOS_AUTH_ERR_OK: Successful description.
 *  - NAOS_AUTH_ERR_NOT_PROVISIONED: If the device is not provisioned.
 *  - NAOS_AUTH_ERR_INVALID_VERSION: If the version in data is not 1.
 *  - NAOS_AUTH_ERR_INVALID_SIGNATURE: If the signature of the data is invalid.
 */
naos_auth_err_t naos_auth_describe(naos_auth_data_t *data);

/**
 * Perform device attestation using a HMAC based challenge-response.
 *
 * @param challenge The challenge data to attest.
 * @param length The length of the challenge data.
 * @param response The response buffer to fill with the HMAC response.
 * @return
 *  - NAOS_AUTH_ERR_OK: Successful authentication.
 *  - NAOS_AUTH_ERR_NOT_PROVISIONED: If the device is not provisioned.
 */
naos_auth_err_t naos_auth_attest(const void *challenge, size_t length, uint8_t response[32]);

#endif  // NAOS_AUTH_H
