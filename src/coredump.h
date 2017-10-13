#ifndef _NAOS_COREDUMP_H
#define _NAOS_COREDUMP_H

/**
 * Gets the total size of the stored core dump.
 *
 * @return Size in bytes or zero when not available.
 */
uint32_t naos_coredump_size();

/**
 * Read a part of the stored core dump.
 *
 * @param offset - The offset from which to read.
 * @param length - The length of the chunk to read.
 * @param buf - The buffer to be filled with data.
 */
void naos_coredump_read(uint32_t offset, uint32_t length, void *buf);

/**
 * Clear any stored coredump.
 */
void naos_coredump_clear();

#endif  // _NAOS_COREDUMP_H
