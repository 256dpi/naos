#include <nadk.h>

#include "system.h"

static nadk_device_t *nadk_device_ref;

void nadk_init(nadk_device_t* device) {
  // set device reference
  nadk_device_ref = device;

  // initialize system
  nadk_system_init();
}

const nadk_device_t *nadk_device() { return nadk_device_ref; }
