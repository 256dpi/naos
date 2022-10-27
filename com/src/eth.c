#include <esp_event.h>
#include <string.h>

#include "utils.h"
#include "net.h"

static SemaphoreHandle_t naos_eth_mutex;
static bool naos_eth_connected = false;
static char naos_eth_ip_addr[16] = {0};

static void naos_eth_handler(void *arg, esp_event_base_t base, int32_t id, void *data) {
  // acquire mutex
  NAOS_LOCK(naos_eth_mutex);

  // handle ethernet events
  if (base == ETH_EVENT) {
    switch (id) {
      case ETHERNET_EVENT_DISCONNECTED: {
        // set status
        naos_eth_connected = false;

        // clear ip addr
        memset(naos_eth_ip_addr, 0, 16);

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled ethernet event: %d", event_id);
      }
    }
  }

  // handle IP events
  if (base == IP_EVENT) {
    switch (id) {
      case IP_EVENT_ETH_GOT_IP: {
        // set status
        naos_eth_connected = true;

        // set ip addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)data;
        sprintf(naos_eth_ip_addr, IPSTR, IP2STR(&event->ip_info.ip));

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled IP event: %d", event_id);
      }
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_eth_mutex);
}

static naos_net_status_t naos_eth_status() {
  // read status
  NAOS_LOCK(naos_eth_mutex);
  naos_net_status_t status = {
      .connected = naos_eth_connected,
  };
  NAOS_UNLOCK(naos_eth_mutex);

  return status;
}

void naos_eth_init() {
  // create mutex
  naos_eth_mutex = xSemaphoreCreateMutex();

  // register event handlers
  ESP_ERROR_CHECK(esp_event_handler_instance_register(ETH_EVENT, ESP_EVENT_ANY_ID, &naos_eth_handler, NULL, NULL));
  ESP_ERROR_CHECK(esp_event_handler_instance_register(IP_EVENT, ESP_EVENT_ANY_ID, &naos_eth_handler, NULL, NULL));

  // register link
  naos_net_link_t link = {.status = naos_eth_status};
  naos_net_register(link);
}
