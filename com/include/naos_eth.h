#ifndef NAOS_ETH_H
#define NAOS_ETH_H

/**
 * Will prepare Ethernet on the Olimex ESP32 Gateway board.
 */
void naos_eth_olimex();

/**
 * Initialize the Ethernet network link.
 *
 * @note Ethernet must be prepared beforehand.
 */
void naos_eth_init();

#endif  // NAOS_ETH_H
