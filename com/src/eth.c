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

#define NAOS_ETH_SAVE_TICK 500     // 0.5s
#define NAOS_ETH_SAVE_GRACE 10000  // 10s
#define NAOS_ETH_SAVE_WINDOW 4000  // 4s

// The W5500 PHY configuration register (PHYCFGR, offset 0x2E in the common
// register block) as addressed by the IDF W5500 driver. Its PHY driver does not
// implement power control, so power saving writes the register directly.
#define NAOS_ETH_W5500_PHYCFGR (0x2E << 16)
#define NAOS_ETH_W5500_PHYCFGR_LNK (1 << 0)
#define NAOS_ETH_W5500_PHYCFGR_RST (1 << 7)
#define NAOS_ETH_W5500_PHYCFGR_DOWN 0x70  // OPMD = 1, OPMDC = 110 (power down)
#define NAOS_ETH_W5500_PHYCFGR_UP 0x78    // OPMD = 1, OPMDC = 111 (all capable, auto-negotiation)

typedef enum {
  NAOS_ETH_POWER_ON,        // PHY powered and driver started
  NAOS_ETH_POWER_OFF,       // PHY powered down and driver stopped
  NAOS_ETH_POWER_CHECKING,  // PHY powered and polled for a link, driver stopped
} naos_eth_power_t;

static naos_mutex_t naos_eth_mutex;
static esp_eth_handle_t naos_eth_handle;
static esp_netif_t *naos_eth_netif;
static bool naos_eth_started = false;
static bool naos_eth_linked = false;
static int64_t naos_eth_unlinked = 0;
static bool naos_eth_connected = false;
static uint32_t naos_eth_generation = 0;
static char naos_eth_addr[16] = {0};
static bool naos_eth_w5500_phy = false;
static uint32_t naos_eth_save_interval = 0;

// only accessed by the defer task (and naos_eth_init before the check timer runs)
static naos_eth_power_t naos_eth_power = NAOS_ETH_POWER_ON;
static int64_t naos_eth_power_since = 0;

static uint32_t naos_eth_phy_read() {
  // read PHYCFGR
  uint32_t value = 0;
  esp_eth_phy_reg_rw_data_t data = {.reg_addr = NAOS_ETH_W5500_PHYCFGR, .reg_value_p = &value};
  ESP_ERROR_CHECK(esp_eth_ioctl(naos_eth_handle, ETH_CMD_READ_PHY_REG, &data));

  return value;
}

static void naos_eth_phy_write(uint32_t value) {
  // write PHYCFGR
  esp_eth_phy_reg_rw_data_t data = {.reg_addr = NAOS_ETH_W5500_PHYCFGR, .reg_value_p = &value};
  ESP_ERROR_CHECK(esp_eth_ioctl(naos_eth_handle, ETH_CMD_WRITE_PHY_REG, &data));
}

static void naos_eth_phy_mode(uint32_t mode) {
  // the mode is only applied by a PHY reset, so write it with the reset bit
  // cleared and set the bit again after the PHY had time to reset
  naos_eth_phy_write(mode);
  naos_delay(10);
  naos_eth_phy_write(mode | NAOS_ETH_W5500_PHYCFGR_RST);
}

static void naos_eth_set_power(naos_eth_power_t power) {
  // set state
  naos_eth_power = power;
  naos_eth_power_since = naos_millis();

  // update parameter
  switch (power) {
    case NAOS_ETH_POWER_ON:
      naos_set_s("eth-power", "on");
      break;
    case NAOS_ETH_POWER_OFF:
      naos_set_s("eth-power", "off");
      break;
    case NAOS_ETH_POWER_CHECKING:
      naos_set_s("eth-power", "checking");
      break;
  }
}

static void naos_eth_stop() {
  // clear state before stopping so published status does not keep a stale address
  naos_lock(naos_eth_mutex);
  bool was_started = naos_eth_started;
  naos_eth_started = false;
  naos_eth_linked = false;
  naos_eth_connected = false;
  memset(naos_eth_addr, 0, 16);
  naos_unlock(naos_eth_mutex);
  naos_set_s("eth-addr", "");

  // stop driver if started
  if (was_started) {
    ESP_ERROR_CHECK(esp_eth_stop(naos_eth_handle));
  }
}

static void naos_eth_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_eth_configure");

  // stop driver
  naos_eth_stop();

  // power up PHY if powered down, the driver fails to start if auto-negotiation
  // is not enabled in PHYCFGR
  if (naos_eth_power == NAOS_ETH_POWER_OFF) {
    naos_eth_phy_mode(NAOS_ETH_W5500_PHYCFGR_UP);
  }

  // configure network
  const char *manual = naos_get_s("eth-manual");
  naos_net_configure(naos_eth_netif, manual);

  // set started flag
  naos_lock(naos_eth_mutex);
  naos_eth_started = true;
  naos_eth_unlinked = naos_millis();
  naos_unlock(naos_eth_mutex);

  // start driver
  ESP_ERROR_CHECK(esp_eth_start(naos_eth_handle));

  // set power
  naos_eth_set_power(NAOS_ETH_POWER_ON);
}

static void naos_eth_check() {
  // get state
  naos_lock(naos_eth_mutex);
  uint32_t interval = naos_eth_save_interval;
  bool linked = naos_eth_linked;
  int64_t unlinked = naos_eth_unlinked;
  naos_unlock(naos_eth_mutex);

  // get time
  int64_t now = naos_millis();

  switch (naos_eth_power) {
    case NAOS_ETH_POWER_ON: {
      // power down if saving and the link has been down for the grace period
      if (interval > 0 && !linked && now - unlinked >= NAOS_ETH_SAVE_GRACE) {
        naos_eth_stop();
        naos_eth_phy_mode(NAOS_ETH_W5500_PHYCFGR_DOWN);
        naos_eth_set_power(NAOS_ETH_POWER_OFF);
      }

      break;
    }

    case NAOS_ETH_POWER_OFF: {
      // power up for good if saving has been disabled
      if (interval == 0) {
        naos_eth_configure();
        break;
      }

      // power up and check for a link once the interval has passed
      if (now - naos_eth_power_since >= interval) {
        naos_eth_phy_mode(NAOS_ETH_W5500_PHYCFGR_UP);
        naos_eth_set_power(NAOS_ETH_POWER_CHECKING);
      }

      break;
    }

    case NAOS_ETH_POWER_CHECKING: {
      // start driver if saving has been disabled or a link came up, this
      // restarts auto-negotiation, which takes another moment
      if (interval == 0 || (naos_eth_phy_read() & NAOS_ETH_W5500_PHYCFGR_LNK) != 0) {
        naos_eth_configure();
        break;
      }

      // power down again if no link came up within the window
      if (now - naos_eth_power_since >= NAOS_ETH_SAVE_WINDOW) {
        naos_eth_phy_mode(NAOS_ETH_W5500_PHYCFGR_DOWN);
        naos_eth_set_power(NAOS_ETH_POWER_OFF);
      }

      break;
    }
  }
}

static void naos_eth_handler(void *arg, esp_event_base_t base, int32_t id, void *data) {
  // handle ethernet events
  if (base == ETH_EVENT) {
    switch (id) {
      case ETHERNET_EVENT_CONNECTED: {
        // check and update state
        naos_lock(naos_eth_mutex);
        if (naos_eth_started) {
          naos_eth_linked = true;
        }
        naos_unlock(naos_eth_mutex);

        break;
      }

      case ETHERNET_EVENT_DISCONNECTED: {
        // check and update state
        naos_lock(naos_eth_mutex);
        if (!naos_eth_started) {
          naos_unlock(naos_eth_mutex);
          break;
        }
        naos_eth_linked = false;
        naos_eth_unlinked = naos_millis();
        naos_eth_connected = false;
        naos_eth_generation++;
        memset(naos_eth_addr, 0, 16);
        naos_unlock(naos_eth_mutex);

        // clear addr
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
        // get addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)data;
        char addr[16] = {0};
        naos_net_ip2str(&event->ip_info.ip, addr);

        // check and update state
        naos_lock(naos_eth_mutex);
        if (!naos_eth_started) {
          naos_unlock(naos_eth_mutex);
          break;
        }
        naos_eth_connected = true;
        naos_eth_generation++;
        memcpy(naos_eth_addr, addr, 16);
        naos_unlock(naos_eth_mutex);

        // set addr
        naos_set_s("eth-addr", addr);

        break;
      }

      case IP_EVENT_ETH_LOST_IP: {
        // check and update state
        naos_lock(naos_eth_mutex);
        if (!naos_eth_started) {
          naos_unlock(naos_eth_mutex);
          break;
        }
        naos_eth_connected = false;
        naos_eth_generation++;
        memset(naos_eth_addr, 0, 16);
        naos_unlock(naos_eth_mutex);

        // clear addr
        naos_set_s("eth-addr", "");

        break;
      }

      default: {
        ESP_LOGD(NAOS_LOG_TAG, "naos_eth_handler: unhandled IP event: %ld", id);
      }
    }
  }
}

static naos_net_status_t naos_eth_status() {
  // read status
  naos_lock(naos_eth_mutex);
  naos_net_status_t status = {
      .active = naos_eth_started,
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
    {.name = "eth-power", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
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

  // enable power saving support
  naos_eth_w5500_phy = true;
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
      .reconfigure = naos_eth_configure,
  };
  naos_net_register(link);

  // register parameters
  for (size_t i = 0; i < NAOS_COUNT(naos_eth_params); i++) {
    naos_register(&naos_eth_params[i]);
  }

  // perform initial configuration
  naos_eth_configure();

  // check power saving periodically
  if (naos_eth_w5500_phy) {
    naos_repeat_defer("naos-eth", NAOS_ETH_SAVE_TICK, naos_eth_check);
  }
}

void naos_eth_power_save(uint32_t interval_ms) {
  // check support
  if (interval_ms > 0 && !naos_eth_w5500_phy) {
    ESP_LOGW(NAOS_LOG_TAG, "naos_eth_power_save: only supported with the W5500");
    return;
  }

  // set interval
  naos_lock(naos_eth_mutex);
  naos_eth_save_interval = interval_ms;
  naos_unlock(naos_eth_mutex);

  // apply change on the defer task, like all other transitions
  if (naos_eth_w5500_phy) {
    naos_defer("naos-eth", 0, naos_eth_check);
  }
}
