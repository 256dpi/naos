#include <esp_eth.h>
#include <esp_eth_phy.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <tcpip_adapter.h>
#include <naos.h>

void eth_init() {
  // prepare mac
  eth_mac_config_t mac_config = ETH_MAC_DEFAULT_CONFIG();
  esp_eth_mac_t *mac = esp_eth_mac_new_esp32(&mac_config);

  // prepare phy
  eth_phy_config_t phy_config = ETH_PHY_DEFAULT_CONFIG();
  esp_eth_phy_t *phy = esp_eth_phy_new_lan87xx(&phy_config);

  // install driver
  esp_eth_config_t config = ETH_DEFAULT_CONFIG(mac, phy);
  esp_eth_handle_t eth_handle = NULL;
  ESP_ERROR_CHECK(esp_eth_driver_install(&config, &eth_handle));

  // start ethernet driver
  ESP_ERROR_CHECK(esp_eth_start(eth_handle));
  //  while (1) {
  //    vTaskDelay(2000 / portTICK_PERIOD_MS);
  //
  //    tcpip_adapter_ip_info_t ip = {0};
  //    if (tcpip_adapter_get_ip_info(TCPIP_ADAPTER_IF_ETH, &ip) == 0) {
  //      naos_log("~~~~~~~~~~~");
  //      naos_log("ETHIP:" IPSTR, IP2STR(&ip.ip));
  //      naos_log("ETHPMASK:" IPSTR, IP2STR(&ip.netmask));
  //      naos_log("ETHPGW:" IPSTR, IP2STR(&ip.gw));
  //      naos_log("~~~~~~~~~~~");
  //    }
  //  }
}
