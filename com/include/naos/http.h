#ifndef NAOS_HTTP_H
#define NAOS_HTTP_H

/**
 * Initialize the HTTP configuration subsystem.
 *
 * @param core The core to run the background task on.
 */
void naos_http_init(int core);

/**
 * Serve a text file with the specified content.
 *
 * @param path The file path.
 * @param type The file type.
 * @param content  The file content.
 */
void naos_http_serve_str(const char *path, const char *type, const char *content);

/**
 * Serve a binary file with the specified content and ecoding.
 *
 * @param path The file path.
 * @param type The file type.
 * @param encoding The file encoding.
 * @param content The file content.
 * @param length The file length.
 */
void naos_http_serve_bin(const char *path, const char *type, const char *encoding, const uint8_t *content, size_t length);

#endif  // NAOS_HTTP_H
