#ifndef _NAOS_MANAGER_H
#define _NAOS_MANAGER_H

#include <naos.h>

/**
 * Initialize the manager subsystem.
 *
 * @note Should only be called once on boot.
 */
void naos_manager_init();

/**
 * Start the manager process.
 */
void naos_manager_start();

/**
 * Handle an incoming message.
 *
 * The message is forwarded to the task if not handled by the manager.
 *
 * @param topic The topic.
 * @param payload The payload.
 * @param len The payload length.
 * @param scope The scope.
 */
void naos_manager_handle(const char* topic, uint8_t* payload, size_t len, naos_scope_t scope);

/**
 * Select a parameter for reading and writing.
 *
 * @param param The parameter.
 */
void naos_manager_select_param(const char* param);

/**
 * Read the previously selected parameter.
 *
 * @return
 */
char* naos_manager_read_param();

/**
 * Write the previously selected parameter.
 *
 * @param value The value.
 */
void naos_manager_write_param(const char* value);

/**
 * Stop the manager process.
 */
void naos_manager_stop();

#endif  // _NAOS_MANAGER_H
