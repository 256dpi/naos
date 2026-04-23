#ifndef NAOS_TZDATA_H
#define NAOS_TZDATA_H

#include <stddef.h>

typedef struct {
  const char *name;
  const char *posix;
} naos_tzdata_entry_t;

extern const naos_tzdata_entry_t naos_tzdata[];
extern const size_t naos_tzdata_count;

#endif  // NAOS_TZDATA_H
