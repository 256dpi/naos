#ifndef _NAOS_NET_H
#define _NAOS_NET_H

/**
 * The connection status emitted by the callback.
 */
typedef enum { NAOS_NET_STATUS_DISCONNECTED, NAOS_NET_STATUS_CONNECTED } naos_net_status_t;

/**
 * The status change callback.
 */
typedef void (*naos_net_status_callback_t)(naos_net_status_t);

/**
 * Initialize the network subsystem.
 *
 * @note Should only be called once on boot.
 *
 * @param callback The status callback.
 */
void naos_net_init(naos_net_status_callback_t callback);

/**
 * Configure the WiFi connection.
 *
 * @note Will automatically disconnect if already connected.
 *
 * @param ssid The WiFi AP SSID.
 * @param password The WiFi AP password.
 */
void naos_wifi_configure(const char *ssid, const char *password);

#endif  // _NAOS_NET_H
