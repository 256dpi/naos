#ifndef NAOS_ETH_H
#define NAOS_ETH_H

#include <driver/gpio.h>

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
 * Initialize the Ethernet network link.
 *
 * @note Ethernet must be prepared beforehand.
 */
void naos_eth_init();

#endif  // NAOS_ETH_H
