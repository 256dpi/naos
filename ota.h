#ifndef _NADK_OTA_H
#define _NADK_OTA_H

#include <stdint.h>

void nadk_ota_init();

void nadk_ota_begin();

void nadk_ota_forward(const char *chunk, uint16_t len);

void nadk_ota_finish();

#endif  // _NADK_OTA_H
