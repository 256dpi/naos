#ifndef NAOS_CPU_H
#define NAOS_CPU_H

/**
 * Initialize the CPU usage monitor.
 */
void naos_cpu_init();


/**
 * Get the CPU usage of the two cores.
 */
void naos_cpu_get(float *cpu0, float *cpu1);

#endif  // NAOS_CPU_H
