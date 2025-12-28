#include <naos.h>
#include <naos/sys.h>
#include <naos/eth.h>

#include <esp_log.h>
#include <driver/spi_master.h>
#include <esp_eth.h>
#include <esp_mac.h>
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
  naos_lock(naos_eth_mutex);

  // stop driver if already started
  if (naos_eth_started) {
    ESP_ERROR_CHECK(esp_eth_stop(naos_eth_handle));
    naos_eth_started = false;
  }

  // set flag
  naos_eth_connected = false;

  // configure network
  const char *manual = naos_get_s("eth-manual");
  naos_net_configure(naos_eth_netif, manual);

  // start driver
  ESP_ERROR_CHECK(esp_eth_start(naos_eth_handle));
  naos_eth_started = true;

  // release mutex
  naos_unlock(naos_eth_mutex);
}

static void naos_eth_handler(void *arg, esp_event_base_t base, int32_t id, void *data) {
  // acquire mutex
  naos_lock(naos_eth_mutex);

  // handle ethernet events
  if (base == ETH_EVENT) {
    switch (id) {
      case ETHERNET_EVENT_DISCONNECTED: {
        // set status
        naos_eth_connected = false;

        // clear addr
        memset(naos_eth_addr, 0, 16);
        naos_set_s("eth-addr", "");

        break;
      }

      default: {
        ESP_LOGD(NAOS_LOG_TAG, "naos_eth_handler: unhandled ethernet event: %ld", id);
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
        naos_set_s("eth-addr", naos_eth_addr);

        break;
      }

      default: {
        ESP_LOGD(NAOS_LOG_TAG, "naos_eth_handler: unhandled IP event: %ld", id);
      }
    }
  }

  // release mutex
  naos_unlock(naos_eth_mutex);
}

static naos_net_status_t naos_eth_status() {
  // read status
  naos_lock(naos_eth_mutex);
  naos_net_status_t status = {
      .connected = naos_eth_connected,
      .generation = naos_eth_generation,
  };
  naos_unlock(naos_eth_mutex);

  return status;
}

static naos_param_t naos_eth_params[] = {
    {.name = "eth-manual", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "eth-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_eth_configure},
    {.name = "eth-addr", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
};

#if defined(CONFIG_IDF_TARGET_ESP32)
void naos_eth_olimex() {
  // prepare EMAC and MAC
  eth_esp32_emac_config_t emac_config = ETH_ESP32_EMAC_DEFAULT_CONFIG();
  eth_mac_config_t mac_config = ETH_MAC_DEFAULT_CONFIG();
  emac_config.clock_config.rmii.clock_mode = EMAC_CLK_OUT;
  emac_config.clock_config.rmii.clock_gpio = EMAC_CLK_OUT_180_GPIO;
  esp_eth_mac_t *mac = esp_eth_mac_new_esp32(&emac_config, &mac_config);

  // prepare PHY
  eth_phy_config_t phy_config = ETH_PHY_DEFAULT_CONFIG();
  phy_config.phy_addr = 0;
  esp_eth_phy_t *phy = esp_eth_phy_new_lan87xx(&phy_config);

  // install driver
  naos_eth_custom(mac, phy);
}
#endif

#if defined(CONFIG_ETH_SPI_ETHERNET_W5500)
void naos_eth_w5500(naos_eth_w5500_t cfg) {
  // ensure GPIO interrupt handler
  esp_err_t err = gpio_install_isr_service(0);
  if (err != ESP_OK && err != ESP_ERR_INVALID_STATE) {
    ESP_ERROR_CHECK(err);
  }

  // initialize SPI bus
  spi_bus_config_t bus_config = {
      .miso_io_num = cfg.miso,
      .mosi_io_num = cfg.mosi,
      .sclk_io_num = cfg.sclk,
      .quadwp_io_num = -1,
      .quadhd_io_num = -1,
  };
  ESP_ERROR_CHECK(spi_bus_initialize(SPI2_HOST, &bus_config, SPI_DMA_CH_AUTO));

  // configure SPI device
  spi_device_interface_config_t device = {
      .mode = 0,
      .command_bits = 16,
      .address_bits = 8,
      .clock_speed_hz = 20 * 1000 * 1000,
      .queue_size = 20,
      .spics_io_num = cfg.select,
  };
  spi_device_handle_t handle = NULL;
  ESP_ERROR_CHECK(spi_bus_add_device(SPI2_HOST, &device, &handle));

  // prepare MAC
  eth_w5500_config_t w5500_config = ETH_W5500_DEFAULT_CONFIG(SPI2_HOST, &device);
  w5500_config.int_gpio_num = cfg.intn;
  eth_mac_config_t mac_config = ETH_MAC_DEFAULT_CONFIG();
  esp_eth_mac_t *mac = esp_eth_mac_new_w5500(&w5500_config, &mac_config);
  if (mac == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // prepare PHY
  eth_phy_config_t phy_config_spi = ETH_PHY_DEFAULT_CONFIG();
  phy_config_spi.phy_addr = 0;
  phy_config_spi.reset_gpio_num = cfg.reset;
  esp_eth_phy_t *phy = esp_eth_phy_new_w5500(&phy_config_spi);
  if (phy == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // install driver
  esp_eth_config_t eth_config = ETH_DEFAULT_CONFIG(mac, phy);
  ESP_ERROR_CHECK(esp_eth_driver_install(&eth_config, &naos_eth_handle));

  // configure MAC address
  uint8_t mac_addr[6] = {0};
  ESP_ERROR_CHECK(esp_read_mac(mac_addr, ESP_MAC_ETH));
  ESP_ERROR_CHECK(esp_eth_ioctl(naos_eth_handle, ETH_CMD_S_MAC_ADDR, mac_addr));

  // create interface
  esp_netif_config_t config = ESP_NETIF_DEFAULT_ETH();
  naos_eth_netif = esp_netif_new(&config);
  if (naos_eth_netif == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // attach interface
  ESP_ERROR_CHECK(esp_netif_attach(naos_eth_netif, esp_eth_new_netif_glue(naos_eth_handle)));
}
#endif

void naos_eth_custom(esp_eth_mac_t *mac, esp_eth_phy_t *phy) {
  // install driver
  esp_eth_config_t config = ETH_DEFAULT_CONFIG(mac, phy);
  ESP_ERROR_CHECK(esp_eth_driver_install(&config, &naos_eth_handle));

  // create interface
  esp_netif_config_t cfg = ESP_NETIF_DEFAULT_ETH();
  naos_eth_netif = esp_netif_new(&cfg);
  if (naos_eth_netif == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // attach interface
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
  for (size_t i = 0; i < NAOS_COUNT(naos_eth_params); i++) {
    naos_register(&naos_eth_params[i]);
  }

  // perform initial configuration
  naos_eth_configure();
}
