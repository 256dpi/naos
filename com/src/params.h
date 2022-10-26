#ifndef NAOS_PARAMS_H
#define NAOS_PARAMS_H

#include <naos.h>

typedef void (*naos_params_receiver_t)(naos_param_t *param);

void naos_params_init();
char *naos_params_list(naos_mode_t mode);
void naos_params_subscribe(naos_params_receiver_t receiver);
void naos_params_dispatch();

// naos_get{b,l,d}
// naos_set{b,l,d}

#endif  // NAOS_PARAMS_H
