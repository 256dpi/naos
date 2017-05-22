#include <nadk.h>

#include "system.h"

static nadk_config_t *nadk_config_ref;

void nadk_init(nadk_config_t *config) {
  // set config reference
  nadk_config_ref = config;

  // initialize system
  nadk_system_init();
}

const nadk_config_t *nadk_config() { return nadk_config_ref; }
