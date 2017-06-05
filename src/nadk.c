#include <nadk.h>
#include <stdio.h>

#include "general.h"
#include "system.h"

static nadk_config_t *nadk_config_ref;

void nadk_init(nadk_config_t *config) {
  // set config reference
  nadk_config_ref = config;

  // initialize system
  nadk_system_init();
}

const nadk_config_t *nadk_config() { return nadk_config_ref; }

void nadk_log(const char *fmt, ...) {
  // prepare args
  va_list args;
  int num;

  // initialize list
  va_start(args, num);

  // process input
  char buf[128];
  vsprintf(buf, fmt, args);

  // print log message esp like
  printf("N (%d) %s: %s\n", nadk_millis(), nadk_config_ref->device_type, buf);

  // free list
  va_end(args);
}
