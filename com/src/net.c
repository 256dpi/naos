#include <esp_event.h>
#include <string.h>

#include "utils.h"
#include "net.h"

#define NAOS_NET_MAX_LINKS 4

static SemaphoreHandle_t naos_net_mutex;
static naos_net_link_t naos_net_links[NAOS_NET_MAX_LINKS] = {0};
static size_t naos_net_link_count = 0;

void naos_net_init() {
  // create mutex
  naos_net_mutex = xSemaphoreCreateMutex();

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

bool naos_net_connected() {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // get status
  bool connected = false;
  for (size_t i = 0; i < naos_net_link_count; i++) {
    naos_net_status_t status = naos_net_links[i].status();
    if (status.connected) {
      connected = true;
      break;
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);

  return connected;
}
