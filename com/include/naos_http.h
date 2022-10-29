#ifndef NAOS_HTTP_H
#define NAOS_HTTP_H

/**
 * Initialize the HTTP configuration subsystem.
 */
void naos_http_init();

/**
 * Install a custom root page.
 */
void naos_http_install(const char *root);

#endif  // NAOS_HTTP_H
