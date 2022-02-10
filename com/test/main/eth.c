#include <driver/gpio.h>
#include <tcpip_adapter.h>
#include <eth_phy/phy_lan8720.h>

#include <naos.h>

#define PIN_SMI_MDC 23
#define PIN_SMI_MDIO 18
#define PIN_PHY_POWER 5

static void eth_phy_power_enable(bool enable) {
  // forward disable
  if (!enable) {
    phy_lan8720_default_ethernet_config.phy_power_enable(false);
  }

  // set GPIO
  gpio_pad_select_gpio(PIN_PHY_POWER);
  gpio_set_direction(PIN_PHY_POWER, GPIO_MODE_OUTPUT);
  gpio_set_level(PIN_PHY_POWER, (int)enable);

  // await GPIO change
  vTaskDelay(1);

  // forward enable
  if (enable) {
    phy_lan8720_default_ethernet_config.phy_power_enable(true);
  }
}

static void eth_gpio_config(void) {
  phy_rmii_configure_data_interface_pins();
  phy_rmii_smi_configure_pins(PIN_SMI_MDC, PIN_SMI_MDIO);
}

void eth_init() {
  // prepare configuration
  eth_config_t config = phy_lan8720_default_ethernet_config;
  config.phy_addr = 0;
  config.gpio_config = eth_gpio_config;
  config.tcpip_input = tcpip_adapter_eth_input;
  config.phy_power_enable = eth_phy_power_enable;
  config.clock_mode = ETH_CLOCK_GPIO17_OUT;

  // initialize and enable ethernet
  ESP_ERROR_CHECK(esp_eth_init(&config));
  ESP_ERROR_CHECK(esp_eth_enable());

  //  while (1) {
  //    vTaskDelay(2000 / portTICK_PERIOD_MS);
  //
  //    tcpip_adapter_ip_info_t ip = {0};
  //    if (tcpip_adapter_get_ip_info(TCPIP_ADAPTER_IF_ETH, &ip) == 0) {
  //      naos_log("~~~~~~~~~~~");
  //      naos_log("ETHIP:"IPSTR, IP2STR(&ip.ip));
  //      naos_log("ETHPMASK:"IPSTR, IP2STR(&ip.netmask));
  //      naos_log("ETHPGW:"IPSTR, IP2STR(&ip.gw));
  //      naos_log("~~~~~~~~~~~");
  //    }
  //  }
}
