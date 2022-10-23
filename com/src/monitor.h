#ifndef NAOS_MONITOR_H
#define NAOS_MONITOR_H

typedef struct {
  float cpu0;
  float cpu1;
} naos_cpu_usage_t;

void naos_monitor_init();
naos_cpu_usage_t naos_monitor_get();

#endif  // NAOS_MONITOR_H
