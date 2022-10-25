#ifndef NAOS_CONFIG_H
#define NAOS_CONFIG_H

typedef enum {
  NAOS_CONFIG_NOTIFICATION_DESCRIPTION,
} naos_config_notification_t;

typedef void (*naos_config_handler_t)(naos_config_notification_t notification);

char* naos_config_describe(bool locked);
char* naos_config_list_params();
char* naos_config_read_param(const char* key);
void naos_config_write_parm(const char* key, const char* value);

void naos_config_register(naos_config_handler_t handler);
void naos_config_notify(naos_config_notification_t notification);

#endif  // NAOS_CONFIG_H
