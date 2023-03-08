#ifndef NAOS_COM_H
#define NAOS_COM_H

#include <naos.h>

typedef struct {
  bool networked;
  uint16_t generation;
} naos_com_status_t;

typedef struct {
  const char *name;
  naos_com_status_t (*status)();
  bool (*subscribe)(const char *topic, int qos);
  bool (*unsubscribe)(const char *topic);
  bool (*publish)(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained);
} naos_com_transport_t;

typedef void (*naos_com_handler_t)(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                   bool retained);

void naos_com_init();
void naos_com_register(naos_com_transport_t transport);
void naos_com_subscribe(naos_com_handler_t handler);
void naos_com_dispatch(const char *topic, const uint8_t *payload, size_t len, int qos, bool retained);
bool naos_com_networked(uint32_t *generation);

// naos_subscribe
// naos_unsubscribe
// naos_publish{s,b,l,d}

#endif  // NAOS_COM_H
