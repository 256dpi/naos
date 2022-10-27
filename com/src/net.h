#ifndef NAOS_NET_H
#define NAOS_NET_H

typedef struct {
  bool connected;
} naos_net_status_t;

typedef struct {
  naos_net_status_t (*status)();
} naos_net_link_t;

void naos_net_init();
void naos_net_register(naos_net_link_t link);
bool naos_net_connected();

#endif  // NAOS_NET_H
