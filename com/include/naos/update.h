#ifndef NAOS_UPDATE_H
#define NAOS_UPDATE_H

#include <stdint.h>
#include <stddef.h>

/**
 * The update status events.
 */
typedef enum {
  NAOS_UPDATE_READY,
  NAOS_UPDATE_DONE,
} naos_update_event_t;

/**
 * The update callback.
 */
typedef void (*naos_update_callback_t)(naos_update_event_t event);

/**
 * Begin a new update. A previously started update will be aborted.
 * The callback will be called when the update is ready/done. If absent,
 * the update will be started synchronously and be ready when this function
 * returns.
 *
 * @param size The size of the update.
 * @param cb The callback to call when the update is ready/done.
 */
void naos_update_begin(size_t size, naos_update_callback_t cb);

/**
 * Write a chunk of data to the update.
 *
 * @param chunk The chunk of data.
 * @param len The length of the chunk.
 */
void naos_update_write(const uint8_t *chunk, size_t len);

/**
 * Abort the update.
 */
void naos_update_abort();

/**
 * Finish the update. If no callback has been set, this function will block
 * until the update is done.
 */
void naos_update_finish();

#endif  // NAOS_UPDATE_H
