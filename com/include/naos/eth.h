#ifndef NAOS_ETH_H
#define NAOS_ETH_H

#include <driver/gpio.h>
#include <esp_eth_mac.h>
#include <esp_eth_phy.h>

typedef struct {
  gpio_num_t mosi;
  gpio_num_t miso;
  gpio_num_t sclk;
  gpio_num_t intn;
  gpio_num_t select;
  gpio_num_t reset;
} naos_eth_w5500_t;

/**
 * Prepare Ethernet on the Olimex ESP32 Gateway board.
 */
void naos_eth_olimex();

/**
 * Prepare Ethernet using a W5500 chip/module.
 */
void naos_eth_w5500(naos_eth_w5500_t config);

/**
 * Prepare a custom ethernet MAC/PHY driver.
 */
void naos_eth_custom(esp_eth_mac_t *mac, esp_eth_phy_t *phy);

/**
 * Initialize the Ethernet network link.
 *
 * @note Ethernet must be prepared beforehand.
 */
void naos_eth_init();

/**
 * Configure Ethernet power saving. When enabled, the PHY is powered down after
 * the link has been down for a while and briefly powered up after each interval
 * to check whether a cable has been connected. The current state is published
 * as "eth-power" ("on", "off" or "checking").
 *
 * @note Only supported with the W5500 and must be called after naos_eth_init().
 *
 * @param interval_ms The time the PHY stays powered down between checks, 0 disables power saving.
 */
void naos_eth_power_save(uint32_t interval_ms);

#endif  // NAOS_ETH_H
