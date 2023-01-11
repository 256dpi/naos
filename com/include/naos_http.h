#ifndef NAOS_HTTP_H
#define NAOS_HTTP_H

/**
 * Initialize the HTTP configuration subsystem.
 *
 * @param core The core to run the background task on.
 */
void naos_http_init(int core);

/**
 * Serve a file with the specified content.
 *
 * @param path The file path.
 * @param type The file type.
 * @param content  The file content.
 */
void naos_http_serve(const char *path, const char *type, const char *content);

#endif  // NAOS_HTTP_H
