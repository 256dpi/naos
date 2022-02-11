#ifndef _NAOS_NET_H
#define _NAOS_NET_H

typedef struct {
  bool connected_any;
  bool connected_wifi;
  bool connected_eth;
} naos_net_status_t;

/**
 * Initialize the network subsystem.
 *
 * @note Should only be called once on boot.
 */
void naos_net_init();

/**
 * Configure the Wi-Fi connection.
 *
 * @note Will automatically disconnect if already connected.
 *
 * @param ssid The Wi-Fi AP SSID.
 * @param password The Wi-Fi AP password.
 */
void naos_net_configure_wifi(const char *ssid, const char *password);

/**
 * Obtain the status of the network subsystem.
 *
 * @return Whether network connectivity is available.
 */
naos_net_status_t naos_net_check();

#endif  // _NAOS_NET_H
