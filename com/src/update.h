#ifndef NAOS_UPDATE_H
#define NAOS_UPDATE_H

#include <stdint.h>

void naos_update_init();
void naos_update_begin(size_t size);
void naos_update_write(uint8_t *chunk, size_t len);
void naos_update_finish();

#endif  // NAOS_UPDATE_H
