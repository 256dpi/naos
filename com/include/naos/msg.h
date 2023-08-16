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
 * mechanism ensures proper isolation for channels that are shared (BLE, MQTT)
 * or stateless (OSC). Security is enforced by the respective channels.
 *
 * The message frame format is defined as follows:
 * | VERSION (1) | SESSION (2) | ENDPOINT (1) | DATA (...) |
 *
 * Endpoints can further define the structure of the remaining data.
 *
 * The system employs three special endpoints: 0x00, 0xFE and 0xFF. The first is
 * used to begin a session and obtain its ID. The second is used to handle pings
 * and report generic acknowledgements and errors. And the last is used to end a
 * session and clean up resources. The message flow is a follows:
 *
 * > Begin: Session=0, Endpoint=0, Data=Handle(*)
 * < Begin: Session=ID, Endpoint=0, Data=Handle(*)
 * > Ping: Session=ID, Endpoint=0xFE
 * < Ack: Session=ID, Endpoint=0xFE
 * < Error: Session=ID, Endpoint=0xFE, Data=Error(1)
 * > End: Session=ID, Endpoint=0xFF
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
 */
typedef struct {
  const char *name;
  size_t mtu;
  bool (*send)(const uint8_t *data, size_t len, void *ctx);
} naos_msg_channel_t;

/**
 * A message error.
 */
typedef enum {
  NAOS_MSG_OK,
  NAOS_MSG_ACK,
  NAOS_MSG_INCOMPLETE,
  NAOS_MSG_INVALID,
  NAOS_MSG_UNKNOWN,
} naos_msg_err_t;

/**
 * A message endpoint.
 */
typedef struct {
  uint8_t ref;
  const char *name;
  naos_msg_err_t (*handle)(naos_msg_t);
  void (*cleanup)(uint16_t session);
} naos_msg_endpoint_t;

/**
 * Initializes the message subsystem.
 */
void naos_msg_init();

/**
 * Registers a channel.
 *
 * @param channel The channel.
 * @return The channel ID.
 */
uint8_t naos_msg_channel_register(naos_msg_channel_t channel);

/**
 * Registers an endpoint.
 *
 * @param endpoint The endpoint.
 */
void naos_msg_endpoint_register(naos_msg_endpoint_t endpoint);

/**
 * Called by channels to dispatch a message.
 *
 * @param channel The channel.
 * @param data The message data.
 * @param len The message length.
 * @param ctx The channel context.
 * @return True if the message was dispatched successfully.
 */
bool naos_msg_channel_dispatch(uint8_t channel, uint8_t *data, size_t len, void *ctx);

/**
 * Called by endpoints to the send a message.
 *
 * @param msg The message.
 * @return True if the message was sent successfully.
 */
bool naos_msg_endpoint_send(naos_msg_t msg);

/**
 * Called by endpoints to determine a sessions channel MTU.
 *
 * @param id The session ID.
 * @return The channel MTU in bytes.
 */
size_t naos_msg_session_mtu(uint16_t id);