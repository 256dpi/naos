#ifndef NAOS_MSG_H
#define NAOS_MSG_H

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

/**
 * NAOS MESSAGE SUBSYSTEM
 * ======================
 *
 * The message subsystem provides a common messaging infrastructure for NAOS
 * components. The system uses multiple sessions over a set of channels that
 * allow communication with as set of system and application endpoints.
 *
 * To communicate with endpoints a session needs to be established. This
 * mechanism ensures proper isolation for shared channels (BLE, MQTT) or
 * stateless (OSC). These sessions are not secured and rely on the security
 * mechanisms provided by the channel (none, TLS, etc.).
 *
 * The message frame format is defined as follows:
 * | VERSION (1) | SESSION (2) | ENDPOINT (1) | DATA (...) |
 *
 * Endpoints can further define the structure of the remaining data.
 *
 * The system has four system endpoints:. The "0x00" endpoint is used to begin a
 * session and obtain its ID. The "0xFE" endpoint is used to handle pings and
 * report generic acknowledgements and errors. And "0xFF" is used to end a
 * session and clean up resources. Existence of endpoints may also be queried by
 * sending an empty message. The message basic flows are as follows:
 *
 * > Begin: Session=0, Endpoint=0, Data=Handle(*)
 * < Begin: Session=ID, Endpoint=0, Data=Handle(*)
 *
 * > Query: Session=ID, Endpoint=7
 * < Reply: Session=ID, Endpoint=0xFE, Data=[ACK|INVALID]
 *
 * > Ping: Session=ID, Endpoint=0xFE
 * < Reply: Session=ID, Endpoint=0xFE, Data=ACK
 *
 * > Command: Session=ID, Endpoint=1, Data=Any(*)
 * < Command: Session=ID, Endpoint=1, Data=Any(*)
 * < Reply: Session=ID, Endpoint=0xFE, Data=[ACK|INVALID|UNKNOWN|ERROR]
 *
 * > End: Session=ID, Endpoint=0xFF
 * < End: Session=ID, Endpoint=0xFF
 *
 * The messaging system provides a basic access control mechanism to prevent
 * endpoints from being accessed by unauthorized sessions. If a device password
 * is set, all sessions are locked by default and need to be unlocked via the
 * system endpoint "0xFD" with the correct password. The system endpoint may also
 * be queried to determine if the session is locked.
 *
 * > Query: Session=ID, Endpoint=0xFD Data=0
 * < Reply: Session=ID, Endpoint=0xFE, Data=[1|0]
 *
 * > Unlock: Session=ID, Endpoint=0xFD, Data=1+Password(*)
 * < Reply: Session=ID, Endpoint=0xFE, Data=[1|0]
 *
 * Messaging channels use different physical transports that support different
 * message sizes. The system provides a mechanism to query the maximum message
 * size for a given session. This allows clients and endpoints to split messages
 * into smaller chunks if necessary. While endpoints can use the C API, remote
 * clients can query the MTU via the system endpoint:
 *
 * > Query: Session=ID, Endpoint=0xFD, Data=2
 * < Reply: Session=ID, Endpoint=0xFE, Data=MTU(2)
 */

/**
 * An incoming or outgoing message.
 */
typedef struct {
  uint16_t session;
  uint8_t endpoint;
  uint8_t *data;
  size_t len;
} naos_msg_t;

/**
 * A message channel.
 *
 * Note: The `mtu` and `send` functions receive the context pointer passed to
 * the `naos_msg_dispatch` function during session creation. The result of the
 * `mtu` function is cached for the session during creation.
 *
 * Note: The channel MTU reflects the underlying maximum message length.
 *
 * @param name The channel name.
 * @param mtu The function to determine the channel MTU.
 * @param send The function to send messages.
 */
typedef struct {
  const char *name;
  uint16_t (*mtu)(void *ctx);
  bool (*send)(const uint8_t *data, size_t len, void *ctx);
} naos_msg_channel_t;

/**
 * A message reply.
 */
typedef enum {
  NAOS_MSG_OK,
  NAOS_MSG_ACK,
  NAOS_MSG_INVALID,
  NAOS_MSG_UNKNOWN,
  NAOS_MSG_ERROR,
  NAOS_MSG_LOCKED,
} naos_msg_reply_t;

// TODO: Ensure that handle is never called after cleanup.

/**
 * A message endpoint.
 *
 * Note: Messages are dispatched by a single background task sequentially.
 *
 * @param ref The endpoint number.
 * @param name The endpoint name.
 * @param handle The function to handle messages.
 * @param cleanup The function to clean up sessions.
 */
typedef struct {
  uint8_t ref;
  const char *name;
  naos_msg_reply_t (*handle)(naos_msg_t);
  void (*cleanup)(uint16_t session);
} naos_msg_endpoint_t;

/**
 * Registers a channel.
 *
 * @param channel The channel.
 * @return The channel ID.
 */
uint8_t naos_msg_register(naos_msg_channel_t channel);

/**
 * Installs an endpoint.
 *
 * @param endpoint The endpoint.
 */
void naos_msg_install(naos_msg_endpoint_t endpoint);

/**
 * Called by channels to dispatch a message.
 *
 * @param channel The channel.
 * @param data The message data.
 * @param len The message length.
 * @param ctx The channel context.
 * @return True if the message was dispatched successfully.
 */
bool naos_msg_dispatch(uint8_t channel, uint8_t *data, size_t len, void *ctx);

/**
 * Called by endpoints to the send a message.
 *
 * @param msg The message.
 * @return True if the message was sent successfully.
 */
bool naos_msg_send(naos_msg_t msg);

/**
 * Called by endpoints to determine a sessions MTU.
 *
 * Note: The message framing overhead is already been subtracted from the value.
 *
 * @param id The session ID.
 * @return The session MTU in bytes.
 */
uint16_t naos_msg_get_mtu(uint16_t id);

/**
 * Called by endpoints to determine if a session is unlocked.
 *
 * @param id The session ID.
 * @return True if the session is unlocked.
 */
bool naos_msg_is_locked(uint16_t id);

#endif  // NAOS_MSG_H
