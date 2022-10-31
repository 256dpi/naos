#ifndef NAOS_UTILS_H
#define NAOS_UTILS_H

#include <stdint.h>
#include <esp_log.h>

#define NAOS_LOG_TAG "naos"

const char *naos_i2str(int32_t num);
const char *naos_d2str(double num);
char *naos_format(char *fmt, ...);
char *naos_concat(const char *str1, const char *str2);

#endif  // NAOS_UTILS_H
