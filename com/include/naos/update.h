#ifndef NAOS_UPDATE_H
#define NAOS_UPDATE_H

#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

/**
 * Begin a new update. Any previously started update will be aborted.
 *
 * @param size The size of the update.
 * @return True if the update was started successfully, false otherwise.
 */
bool naos_update_begin(size_t size);

/**
 * Write a chunk of data to the update.
 *
 * @param chunk The chunk of data.
 * @param len The length of the chunk.
 * @return True if the chunk was written successfully, false otherwise.
 */
bool naos_update_write(const uint8_t *chunk, size_t len);

/**
 * Abort the update.
 *
 * @return True if the update was aborted successfully, false otherwise.
 */
bool naos_update_abort();

/**
 * Finish the update.
 *
 * @return True if the update was finished successfully, false otherwise.
 */
bool naos_update_finish();

#endif  // NAOS_UPDATE_H
