#ifndef NAOS_MANAGER_H
#define NAOS_MANAGER_H

#include <naos.h>

void naos_manager_init();
void naos_manager_start();
void naos_manager_handle(const char* topic, uint8_t* payload, size_t len, naos_scope_t scope);
void naos_manager_stop();

// naos_log

#endif  // NAOS_MANAGER_H
