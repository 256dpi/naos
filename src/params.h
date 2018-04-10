/**
 * Initialize parameter management.
 *
 * Note: Should only be called once on boot.
 */
void naos_params_init();

/**
 * Trigger a parameter synchoronization.
 *
 * @param param - The parameter.
 */
void naos_params_sync(const char *param);
