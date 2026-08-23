#ifndef _NAOS_UTILS_H
#define _NAOS_UTILS_H

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#define NAOS_LOG_TAG "naos"

void *naos_alloc(size_t size);
const char *naos_i2str(char buf[16], int32_t num);
const char *naos_d2str(char buf[32], double num);
char *naos_format(const char *fmt, ...);
uint8_t *naos_copy(uint8_t *buf, size_t len);
char *naos_concat(const char *str1, const char *str2);
bool naos_equal(uint8_t *buf, size_t len, const char *str);

#endif  // _NAOS_UTILS_H
