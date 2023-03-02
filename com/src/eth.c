#include <naos.h>
#include <naos/sys.h>

#include <esp_event.h>
#include <string.h>

#include "utils.h"
#include "net.h"

static naos_mutex_t naos_eth_mutex;
static esp_eth_handle_t naos_eth_handle;
static esp_netif_t *naos_eth_netif;
static bool naos_eth_started = false;
static bool naos_eth_connected = false;
static uint16_t naos_eth_generation = 0;
static char naos_eth_addr[16] = {0};

static void naos_eth_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_eth_configure");

  // acquire mutex
  NAOS_LOCK(naos_eth_mutex);

  // stop driver if already started
  if (naos_eth_started) {
    ESP_ERROR_CHECK(esp_eth_stop(naos_eth_handle));
    naos_eth_started = false;
  }

  // set flag
  naos_eth_connected = false;

  // configure network
  const char *manual = naos_get("eth-manual");
  naos_net_configure(naos_eth_netif, manual);

  // start driver
  ESP_ERROR_CHECK(esp_eth_start(naos_eth_handle));
  naos_eth_started = true;

  // release mutex
  NAOS_UNLOCK(naos_eth_mutex);
}

static void naos_eth_handler(void *arg, esp_event_base_t base, int32_t id, void *data) {
  // acquire mutex
  NAOS_LOCK(naos_eth_mutex);

  // handle ethernet events
  if (base == ETH_EVENT) {
    switch (id) {
      case ETHERNET_EVENT_DISCONNECTED: {
        // set status
        naos_eth_connected = false;

        // clear addr
        memset(naos_eth_addr, 0, 16);
        naos_set("eth-addr", "");

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
        naos_eth_generation++;

        // set addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)data;
        naos_net_ip2str(&event->ip_info.ip, naos_eth_addr);
        naos_set("eth-addr", naos_eth_addr);

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
      .generation = naos_eth_generation,
  };
  NAOS_UNLOCK(naos_eth_mutex);

  return status;
}

static naos_param_t naos_eth_params[] = {
    {.name = "eth-manual", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "eth-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_eth_configure},
    {.name = "eth-addr", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
};

void naos_eth_olimex() {
  // prepare mac
  eth_mac_config_t mac_config = ETH_MAC_DEFAULT_CONFIG();
  mac_config.clock_config.rmii.clock_mode = EMAC_CLK_OUT;
  mac_config.clock_config.rmii.clock_gpio = EMAC_CLK_OUT_180_GPIO;
  esp_eth_mac_t *mac = esp_eth_mac_new_esp32(&mac_config);

  // prepare phy
  eth_phy_config_t phy_config = ETH_PHY_DEFAULT_CONFIG();
  phy_config.phy_addr = 0;
  esp_eth_phy_t *phy = esp_eth_phy_new_lan87xx(&phy_config);

  // install driver
  esp_eth_config_t config = ETH_DEFAULT_CONFIG(mac, phy);
  ESP_ERROR_CHECK(esp_eth_driver_install(&config, &naos_eth_handle));

  // create interface
  esp_netif_config_t cfg = ESP_NETIF_DEFAULT_ETH();
  naos_eth_netif = esp_netif_new(&cfg);

  // attach ethernet
  ESP_ERROR_CHECK(esp_netif_attach(naos_eth_netif, esp_eth_new_netif_glue(naos_eth_handle)));
}

void naos_eth_init() {
  // create mutex
  naos_eth_mutex = naos_mutex();

  // register event handlers
  ESP_ERROR_CHECK(esp_event_handler_instance_register(ETH_EVENT, ESP_EVENT_ANY_ID, &naos_eth_handler, NULL, NULL));
  ESP_ERROR_CHECK(esp_event_handler_instance_register(IP_EVENT, ESP_EVENT_ANY_ID, &naos_eth_handler, NULL, NULL));

  // register link
  naos_net_link_t link = {
      .name = "eth",
      .status = naos_eth_status,
  };
  naos_net_register(link);

  // register parameters
  for (size_t i = 0; i < (sizeof(naos_eth_params) / sizeof(naos_eth_params[0])); i++) {
    naos_register(&naos_eth_params[i]);
  }

  // perform initial configuration
  naos_eth_configure();
}
