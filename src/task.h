#ifndef _NADK_TASK_H
#define _NADK_TASK_H

#include <nadk.h>

/**
 * Initialize the task subsystem.
 *
 * Note: Should only be called once on boot.
 */
void nadk_task_init();

/**
 * Start the task process.
 */
void nadk_task_start();

/**
 * Stop the task process.
 */
void nadk_task_stop();

/**
 * Notify the task about a status change.
 *
 * @param status
 */
void nadk_task_notify(nadk_status_t status);

/**
 * Forward a message to the task process.
 *
 * @param topic - The topic.
 * @param payload - The payload.
 * @param len - The payload length.
 * @param scope - The scope.
 */
void nadk_task_forward(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope);

#endif  // _NADK_TASK_H
