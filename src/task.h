#ifndef _NAOS_TASK_H
#define _NAOS_TASK_H

#include <naos.h>

/**
 * Initialize the task subsystem.
 *
 * Note: Should only be called once on boot.
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

/**
 * Notify the task about a status change.
 *
 * @param status
 */
void naos_task_notify(naos_status_t status);

/**
 * Pass in a parameter update.
 */
void naos_task_update(const char *param, const char *value);

/**
 * Forward a message to the task process.
 *
 * @param topic - The topic.
 * @param payload - The payload.
 * @param len - The payload length.
 * @param scope - The scope.
 */
void naos_task_forward(const char *topic, const char *payload, unsigned int len, naos_scope_t scope);

#endif  // _NAOS_TASK_H
