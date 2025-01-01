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
