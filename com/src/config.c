#include "config.h"
#include "manager.h"

// TODO: Directly write params.
//  => Manager should also subscribe to params for updates.

void naos_config_write_param(const char* key, const char* value) {
  // write param
  return naos_manager_write_param(naos_lookup(key), value);
}
