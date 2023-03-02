#ifndef NAOS_UPDATE_H
#define NAOS_UPDATE_H

#include <stdint.h>

typedef enum {
  NAOS_UPDATE_READY,
  NAOS_UPDATE_DONE,
} naos_update_event_t;

typedef void (*naos_update_callback_t)(naos_update_event_t event);

void naos_update_init();
void naos_update_begin(size_t size, naos_update_callback_t cb);
void naos_update_write(const uint8_t *chunk, size_t len);
void naos_update_finish();

#endif  // NAOS_UPDATE_H
