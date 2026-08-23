#include <naos/sys.h>

#include <esp_event.h>
#include <esp_log.h>
#include <string.h>

#include "net.h"
#include "utils.h"

#define NAOS_NET_MAX_LINKS 4
#define NAOS_NET_WATCHDOG_INTERVAL 5000     // 5s
#define NAOS_NET_WATCHDOG_TIMEOUT 300000    // 5m

static naos_mutex_t naos_net_mutex;
static naos_net_link_t naos_net_links[NAOS_NET_MAX_LINKS] = {0};
static size_t naos_net_link_count = 0;
static uint32_t naos_net_seen_generation[NAOS_NET_MAX_LINKS] = {0};
static int64_t naos_net_quiet_since[NAOS_NET_MAX_LINKS] = {0};

static void naos_net_watchdog() {
  // a healthy link that cannot connect keeps producing status changes as the
  // driver retries, so a link that claims to be active, is not connected and
  // has been completely quiet for the timeout has a stalled driver and gets
  // reconfigured to restore its event and retry machinery

  // collect stalled links
  naos_net_link_t stalled[NAOS_NET_MAX_LINKS];
  size_t num_stalled = 0;
  int64_t now = naos_millis();
  naos_lock(naos_net_mutex);
  for (size_t i = 0; i < naos_net_link_count; i++) {
    // skip links without reconfigure support
    naos_net_link_t *link = &naos_net_links[i];
    if (link->reconfigure == NULL) {
      continue;
    }

    // get status
    naos_net_status_t status = link->status();

    // disarm if inactive or connected
    if (!status.active || status.connected) {
      naos_net_quiet_since[i] = 0;
      naos_net_seen_generation[i] = status.generation;
      continue;
    }

    // re-arm on first observation or status change
    if (naos_net_quiet_since[i] == 0 || status.generation != naos_net_seen_generation[i]) {
      naos_net_quiet_since[i] = now;
      naos_net_seen_generation[i] = status.generation;
      continue;
    }

    // collect link if quiet for too long
    if (now - naos_net_quiet_since[i] >= NAOS_NET_WATCHDOG_TIMEOUT) {
      stalled[num_stalled] = *link;
      num_stalled++;
      naos_net_quiet_since[i] = now;
    }
  }
  naos_unlock(naos_net_mutex);

  // reconfigure stalled links
  for (size_t i = 0; i < num_stalled; i++) {
    ESP_LOGW(NAOS_LOG_TAG, "naos_net_watchdog: reconfiguring stalled link '%s'", stalled[i].name);
    stalled[i].reconfigure();
  }
}

void naos_net_init() {
  // create mutex
  naos_net_mutex = naos_mutex();

  // initialize networking
  ESP_ERROR_CHECK(esp_netif_init());

  // create default event loop
  ESP_ERROR_CHECK(esp_event_loop_create_default());

  // start watchdog
  naos_repeat_defer("naos-net", NAOS_NET_WATCHDOG_INTERVAL, naos_net_watchdog);
}

void naos_net_register(naos_net_link_t link) {
  // acquire mutex
  naos_lock(naos_net_mutex);

  // check count
  if (naos_net_link_count >= NAOS_NET_MAX_LINKS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store link
  naos_net_links[naos_net_link_count] = link;
  naos_net_link_count++;

  // release mutex
  naos_unlock(naos_net_mutex);
}

bool naos_net_connected(uint32_t *generation) {
  // acquire mutex
  naos_lock(naos_net_mutex);

  // get status
  bool connected = false;
  uint32_t current_generation = 0;
  for (size_t i = 0; i < naos_net_link_count; i++) {
    naos_net_status_t status = naos_net_links[i].status();
    current_generation += status.generation;
    if (status.connected) {
      connected = true;
    }
  }

  // update generation
  if (generation != NULL) {
    *generation = current_generation;
  }

  // release mutex
  naos_unlock(naos_net_mutex);

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
  // parse manual config first so invalid values can fall back to DHCP
  char addr[16] = {0};
  char gateway[16] = {0};
  char mask[16] = {0};
  bool manual = sscanf(config, "%15[^,],%15[^,],%15[^,]", addr, gateway, mask) == 3;
  esp_netif_ip_info_t info = {0};
  if (manual && !(naos_net_str2ip(addr, &info.ip) && naos_net_str2ip(gateway, &info.gw) &&
                  naos_net_str2ip(mask, &info.netmask))) {
    ESP_LOGW(NAOS_LOG_TAG, "naos_net_configure: invalid manual config '%s', falling back to DHCP", config);
    manual = false;
  }

  // stop DHCP if not stopped
  esp_netif_dhcp_status_t status = {0};
  ESP_ERROR_CHECK(esp_netif_dhcpc_get_status(netif, &status));
  if (status != ESP_NETIF_DHCP_STOPPED) {
    ESP_ERROR_CHECK(esp_netif_dhcpc_stop(netif));
  }

  if (manual) {
    // configure manual
    ESP_ERROR_CHECK(esp_netif_set_ip_info(netif, &info));
  } else {
    // configure automatic
    esp_netif_ip_info_t auto_info = {0};
    ESP_ERROR_CHECK(esp_netif_set_ip_info(netif, &auto_info));

    // start DHCP
    ESP_ERROR_CHECK(esp_netif_dhcpc_start(netif));
  }
}
