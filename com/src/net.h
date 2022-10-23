#ifndef _NAOS_SETTINGS_H
#define _NAOS_SETTINGS_H

typedef struct {
  bool started_wifi;
  bool connected_any;
  bool connected_wifi;
  bool connected_eth;
  char ip_wifi[16];
  char ip_eth[16];
} naos_net_status_t;

void naos_net_init();
void naos_net_configure_wifi(const char *ssid, const char *password);
naos_net_status_t naos_net_check();

#endif  // _NAOS_SETTINGS_H
