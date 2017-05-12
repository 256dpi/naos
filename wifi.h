#ifndef _NADK_WIFI_H
#define _NADK_WIFI_H

/**
 * The connection status emitted by the callback.
 */
typedef enum nadk_wifi_status_t { NADK_WIFI_STATUS_DISCONNECTED, NADK_WIFI_STATUS_CONNECTED } nadk_wifi_status_t;

/**
 * The status change callback.
 */
typedef void (*nadk_wifi_callback_t)(nadk_wifi_status_t);

/**
 * Initialize the WiFi management system.
 *
 * Note: Should only be called once on boot.
 *
 * @param callback
 */
void nadk_wifi_init(nadk_wifi_callback_t callback);

/**
 * Reconfigure the WiFI connection.
 *
 * @param ssid
 * @param password
 */
void nadk_wifi_configure(const char *ssid, const char *password);

#endif  // _NADK_WIFI_H
