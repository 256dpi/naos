#ifndef NAOS_COM_H
#define NAOS_COM_H

#include <naos.h>

typedef struct {
  bool networked;
  uint16_t generation;
} naos_com_status_t;

typedef struct {
  naos_com_status_t (*status)();
  bool (*subscribe)(naos_scope_t scope, const char *topic, int qos);
  bool (*unsubscribe)(naos_scope_t scope, const char *topic);
  bool (*publish)(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos, bool retained);
} naos_com_transport_t;

typedef void (*naos_com_receiver_t)(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                                    bool retained);

void naos_com_init();
void naos_com_register(naos_com_transport_t transport);
void naos_com_subscribe(naos_com_receiver_t receiver);
void naos_com_dispatch(naos_scope_t scope, const char *topic, const uint8_t *payload, size_t len, int qos,
                       bool retained);
bool naos_com_networked(uint32_t *generation);

// naos_subscribe
// naos_unsubscribe
// naos_publish{b,l,d,r}

#endif  // NAOS_COM_H
