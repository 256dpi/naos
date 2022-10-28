#ifndef NAOS_NET_H
#define NAOS_NET_H

#include <esp_netif.h>

typedef struct {
  bool connected;
} naos_net_status_t;

typedef struct {
  naos_net_status_t (*status)();
} naos_net_link_t;

void naos_net_init();
void naos_net_register(naos_net_link_t link);
bool naos_net_connected();

bool naos_net_ip2str(esp_ip4_addr_t *addr, char str[16]);
bool naos_net_str2ip(char str[16], esp_ip4_addr_t *addr);
void naos_net_configure(esp_netif_t *netif, const char *config);

#endif  // NAOS_NET_H
