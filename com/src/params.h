/**
 * Initialize parameter management.
 *
 * @note Should only be called once on boot.
 */
void naos_params_init();

/**
 * Create a comma separated list of parameter:type pairs.
 *
 * @note Returned pointer must be freed after usage.
 *
 * @return Pointer to list.
 */
char *naos_params_list();
