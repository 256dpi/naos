#ifndef NAOS_WIFI_H
#define NAOS_WIFI_H

/**
 * Initialize the WiFi network link.
 */
void naos_wifi_init();

/**
 * Get the WiFi information.
 */
void naos_wifi_info(int8_t * rssi);

#endif  // NAOS_WIFI_H
