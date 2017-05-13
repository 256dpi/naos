#ifndef _NADK_OTA_H
#define _NADK_OTA_H

#include <stdint.h>

/**
 * Initialize the OTA management system.
 *
 * Note: Should only be called once on boot.
 */
void nadk_ota_init();

/**
 * Begin with an OTA update.
 */
void nadk_ota_begin();

/**
 * Forward an incoming chunk of the new firmware image.
 *
 * @param chunk
 * @param len
 */
void nadk_ota_forward(const char *chunk, uint16_t len);

/**
 * Finish an OTA update.
 */
void nadk_ota_finish();

#endif  // _NADK_OTA_H
