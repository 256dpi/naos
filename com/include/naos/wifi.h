#ifndef NAOS_WIFI_H
#define NAOS_WIFI_H

/**
 * Initialize the WiFi network link.
 */
void naos_wifi_init();

/**
 * Get the WiFi RSSI in dBm.
 *
 * Returns `-128` if the station is currently disconnected and no RSSI is
 * available.
 */
void naos_wifi_info(int8_t * rssi);

#endif  // NAOS_WIFI_H
