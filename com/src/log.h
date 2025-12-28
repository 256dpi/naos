#ifndef _NAOS_LOG_H
#define _NAOS_LOG_H

typedef void (*naos_log_sink_t)(const char *msg);

void naos_log_init();
void naos_log_register(naos_log_sink_t sink);

// naos_log

#endif  // _NAOS_LOG_H
