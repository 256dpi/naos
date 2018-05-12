#ifndef _NAOS_WIFI_H
#define _NAOS_WIFI_H

/**
 * The connection status emitted by the callback.
 */
typedef enum { NAOS_WIFI_STATUS_DISCONNECTED, NAOS_WIFI_STATUS_CONNECTED } naos_wifi_status_t;

/**
 * The status change callback.
 */
typedef void (*naos_wifi_status_callback_t)(naos_wifi_status_t);

/**
 * Initialize the WiFi subsystem.
 *
 * @note Should only be called once on boot.
 *
 * @param callback The status callback.
 */
void naos_wifi_init(naos_wifi_status_callback_t callback);

/**
 * Configure the WiFi connection.
 *
 * @note Will automatically disconnect if already connected.
 *
 * @param ssid The WiFi AP SSID.
 * @param password The WiFi AP password.
 */
void naos_wifi_configure(const char *ssid, const char *password);

#endif  // _NAOS_WIFI_H
