#include <naos/sys.h>

#include <esp_event.h>
#include <string.h>

#include "net.h"

#define NAOS_NET_MAX_LINKS 4

static naos_mutex_t naos_net_mutex;
static naos_net_link_t naos_net_links[NAOS_NET_MAX_LINKS] = {0};
static size_t naos_net_link_count = 0;

void naos_net_init() {
  // create mutex
  naos_net_mutex = naos_mutex();

  // initialize networking
  ESP_ERROR_CHECK(esp_netif_init());

  // create default event loop
  ESP_ERROR_CHECK(esp_event_loop_create_default());
}

void naos_net_register(naos_net_link_t link) {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // check count
  if (naos_net_link_count >= NAOS_NET_MAX_LINKS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store link
  naos_net_links[naos_net_link_count] = link;
  naos_net_link_count++;

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);
}

bool naos_net_connected(uint32_t *generation) {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // get status
  bool connected = false;
  for (size_t i = 0; i < naos_net_link_count; i++) {
    naos_net_status_t status = naos_net_links[i].status();
    if (status.connected) {
      connected = true;
      if (generation != NULL) {
        *generation += (uint32_t)status.generation;
      }
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);

  return connected;
}

bool naos_net_ip2str(esp_ip4_addr_t *addr, char str[16]) {
  int ret = snprintf(str, 16, IPSTR, IP2STR(addr));
  return ret > 0 && ret < 16;
}

bool naos_net_str2ip(char str[16], esp_ip4_addr_t *addr) {
  int a, b, c, d;
  if (sscanf(str, IPSTR, &a, &b, &c, &d) != 4) {
    return false;
  }
  addr->addr = ESP_IP4TOADDR(a, b, c, d);
  return true;
}

void naos_net_configure(esp_netif_t *netif, const char *config) {
  // stop DHCP if not stopped
  esp_netif_dhcp_status_t status = {0};
  ESP_ERROR_CHECK(esp_netif_dhcpc_get_status(netif, &status));
  if (status != ESP_NETIF_DHCP_STOPPED) {
    ESP_ERROR_CHECK(esp_netif_dhcpc_stop(netif));
  }

  // handle config
  char addr[16] = {0};
  char gateway[16] = {0};
  char mask[16] = {0};
  if (sscanf(config, "%15[^,],%15[^,],%15[^,]", addr, gateway, mask) == 3) {
    // configure manual
    esp_netif_ip_info_t info = {0};
    if (naos_net_str2ip(addr, &info.ip) && naos_net_str2ip(gateway, &info.gw) && naos_net_str2ip(mask, &info.netmask)) {
      ESP_ERROR_CHECK(esp_netif_set_ip_info(netif, &info));
    }
  } else {
    // configure automatic
    esp_netif_ip_info_t info = {0};
    ESP_ERROR_CHECK(esp_netif_set_ip_info(netif, &info));

    // start DHCP
    ESP_ERROR_CHECK(esp_netif_dhcpc_start(netif));
  }
}
