#ifndef NAOS_HTTP_H
#define NAOS_HTTP_H

/**
 * The HTTP service configuration.
 */
typedef struct {
  // The core to run the background task on.
  int core;

  // Disable TCP keep-alive on client connections. By default keep-alive is
  // enabled so a client that vanishes without a clean close is detected and its
  // socket reclaimed; set this to keep the previous always-persist behaviour.
  bool no_keep_alive;
} naos_http_config_t;

/**
 * Initialize the HTTP configuration subsystem.
 *
 * @param config The configuration.
 */
void naos_http_init(naos_http_config_t config);

/**
 * Serve a text file with the specified content.
 *
 * @param path The file path or NULL for all paths.
 * @param type The file type.
 * @param content  The file content.
 */
void naos_http_serve_str(const char *path, const char *type, const char *content);

/**
 * Serve a binary file with the specified content and encoding.
 *
 * @param path The file path or NULL for all paths.
 * @param type The file type.
 * @param encoding The file encoding.
 * @param content The file content.
 * @param length The file length.
 */
void naos_http_serve_bin(const char *path, const char *type, const char *encoding, const uint8_t *content, size_t length);

#endif  // NAOS_HTTP_H
