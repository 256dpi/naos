#ifndef NAOS_HTTP_H
#define NAOS_HTTP_H

/**
 * Initialize the HTTP configuration subsystem.
 */
void naos_http_init();

/**
 * Serve a file with the specified content.
 *
 * @param path The file path.
 * @param content  The file content.
 */
void naos_http_serve(const char *path, const char *content);

#endif  // NAOS_HTTP_H
