#ifndef _NAOS_SYSTEM_H
#define _NAOS_SYSTEM_H

#include <naos.h>

typedef void (*naos_system_handler_t)(naos_status_t status);

void naos_system_init();
void naos_system_subscribe(naos_system_handler_t handler);

// naos_status

#endif  // _NAOS_SYSTEM_H
