#ifndef NAOS_MDNS_H
#define NAOS_MDNS_H

/**
 * The mDNS service configuration.
 */
typedef struct {
  bool main;
  bool http;
  bool osc;
} naos_mdns_config_t;

/**
 * Initializes the mDNS service.
 */
void naos_mdns_init(naos_mdns_config_t config);

#endif  // NAOS_MDNS_H
