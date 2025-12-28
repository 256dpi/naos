#ifndef _NAOS_PARAMS_H
#define _NAOS_PARAMS_H

#include <naos.h>

typedef void (*naos_params_handler_t)(naos_param_t *param);

void naos_params_init();
char *naos_params_list(naos_mode_t mode);
void naos_params_subscribe(naos_params_handler_t handler);
void naos_params_dispatch();

// naos_register
// naos_lookup
// naos_get{s,b,l,d}
// naos_set{s,b,l,d}

#endif  // _NAOS_PARAMS_H
