#ifndef NAOS_DEBUG_H
#define NAOS_DEBUG_H

/**
 * Install the debug endpoint.
 */
void naos_debug_install();

/* Coredump Helpers */

/**
 * Get the size of the coredump.
 *
 * @return Size of the coredump in bytes or 0 if none exists.
 */
uint32_t naos_debug_cdp_size();

/**
 * Get the reason for the coredump.
 *
 * @param buf Buffer to write reason into.
 * @param len Length of the buffer.
 * @return true if reason was written, false if no coredump exists.
 */
bool naos_debug_cdp_reason(char *buf, size_t len);

/**
 * Read data from the coredump.
 *
 * @param offset Offset to read from.
 * @param length Length of data to read.
 * @param buf Buffer to read data into.
 */
void naos_debug_cdp_read(uint32_t offset, uint32_t length, void *buf);

/**
 * Delete the coredump.
 */
void naos_debug_cdp_delete();

#endif // NAOS_DEBUG_H
