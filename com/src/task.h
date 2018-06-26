#ifndef _NAOS_TASK_H
#define _NAOS_TASK_H

#include <naos.h>

/**
 * Initialize the task subsystem.
 *
 * @note Should only be called once on boot.
 */
void naos_task_init();

/**
 * Start the task process.
 */
void naos_task_start();

/**
 * Stop the task process.
 */
void naos_task_stop();

#endif  // _NAOS_TASK_H
