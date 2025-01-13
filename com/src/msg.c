#include <naos.h>
#include <naos/sys.h>
#include <naos/msg.h>

#include <string.h>

#include "utils.h"

#define NAOS_MSG_DEBUG CONFIG_NAOS_MSG_DEBUG
#define NAOS_MSG_MAX_CHANNELS 8
#define NAOS_MSG_MAX_ENDPOINTS 32
#define NAOS_MSG_MAX_SESSIONS 64

typedef struct {
  bool active;
  uint16_t id;
  size_t channel;
  void* context;
  int64_t last_msg;
  bool locked;
} naos_msg_session_t;

typedef enum {
  NAOS_MSG_SYS_STATUS_LOCKED = 1 << 0,
} naos_msg_sys_status_t;

typedef enum {
  NAOS_MSG_SYS_CMD_STATUS,
  NAOS_MSG_SYS_CMD_UNLOCK,
} naos_msg_sys_cmd_t;

static naos_mutex_t naos_msg_mutex;
static naos_queue_t naos_msg_queue;
static naos_msg_channel_t naos_msg_channels[NAOS_MSG_MAX_CHANNELS] = {0};
static size_t naos_msg_channel_count = 0;
static naos_msg_endpoint_t naos_msg_endpoints[NAOS_MSG_MAX_ENDPOINTS] = {0};
static size_t naos_msg_endpoint_count = 0;
static naos_msg_session_t naos_msg_session[NAOS_MSG_MAX_SESSIONS] = {0};
static uint16_t naos_msg_next_session = 1;

static naos_msg_session_t* naos_msg_find(uint16_t id) {
  // find matching and active session
  for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
    naos_msg_session_t* s = &naos_msg_session[i];
    if (s->id == id) {
      return s->active ? s : NULL;
    }
  }

  return NULL;
}

static void naos_msg_worker() {
  for (;;) {
    // await message
    naos_msg_t msg;
    naos_pop(naos_msg_queue, &msg, -1);

    // acquire mutex
    NAOS_LOCK(naos_msg_mutex);

    // find endpoint
    naos_msg_endpoint_t* endpoint = NULL;
    for (size_t i = 0; i < naos_msg_endpoint_count; i++) {
      if (naos_msg_endpoints[i].ref == msg.endpoint) {
        endpoint = &naos_msg_endpoints[i];
        break;
      }
    }

    // release mutex
    NAOS_UNLOCK(naos_msg_mutex);

    // skip if endpoint not found
    if (endpoint == NULL) {
      free(msg.data);
      continue;
    }

    // we do not hold the mutex when calling into the endpoint as the callee
    // most likely calls back into us to send a message to the session

    // handle message
    naos_msg_reply_t reply = endpoint->handle(msg);

    // send non-ok replies
    if (reply != NAOS_MSG_OK) {
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = 0xFE,
          .data = (uint8_t*)&reply,
          .len = 1,
      });
    }

    // free data
    free(msg.data);
  }
}

static void naos_msg_cleaner() {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // get current time
  int64_t now = naos_millis();

  // iterate over sessions
  for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
    // get session
    naos_msg_session_t* session = &naos_msg_session[i];

    // skip inactive or young session
    if (!session->active || now - session->last_msg < 30000) {
      continue;
    }

    // log error
    ESP_LOGE("MSG", "naos_msg_cleaner: session %d timed out", session->id);

    // clean up endpoints
    for (size_t j = 0; j < naos_msg_endpoint_count; j++) {
      if (naos_msg_endpoints[j].cleanup != NULL) {
        naos_msg_endpoints[j].cleanup(session->id);
      }
    }

    // clear session
    *session = (naos_msg_session_t){0};
  }

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);
}

static naos_msg_reply_t naos_msg_process_system(naos_msg_t msg) {
  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // get command
  naos_msg_sys_cmd_t cmd = msg.data[0];

  // adjust message
  msg.data++;
  msg.len--;

  // handle command
  switch (cmd) {
    case NAOS_MSG_SYS_CMD_STATUS: {
      // check length
      if (msg.len != 0) {
        return NAOS_MSG_INVALID;
      }

      // prepare status
      uint8_t status = 0;
      if (naos_msg_is_locked(msg.session)) {
        status |= NAOS_MSG_SYS_STATUS_LOCKED;
      }

      // send status
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = 0xFD,
          .data = &status,
          .len = 1,
      });

      return NAOS_MSG_OK;
    }

    case NAOS_MSG_SYS_CMD_UNLOCK: {
      // check length
      if (msg.len == 0) {
        return NAOS_MSG_INVALID;
      }

      // check lock status
      if (!naos_msg_is_locked(msg.session)) {
        return NAOS_MSG_ERROR;
      }

      // check password
      bool ok = naos_equal(msg.data, msg.len, naos_get_s("device-password"));

      // unlock session if correct
      if (ok) {
        NAOS_LOCK(naos_msg_mutex);
        naos_msg_session_t* session = naos_msg_find(msg.session);
        session->locked = false;
        NAOS_UNLOCK(naos_msg_mutex);
      }

      // send result
      uint8_t result = ok ? 1 : 0;
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = 0xFD,
          .data = &result,
          .len = 1,
      });

      return NAOS_MSG_OK;
    }

    default:
      return NAOS_MSG_UNKNOWN;
  }
}

void naos_msg_init() {
  // create mutex
  naos_msg_mutex = naos_mutex();

  // create queue
  naos_msg_queue = naos_queue(NAOS_MSG_MAX_SESSIONS, sizeof(naos_msg_t));

  // run worker
  naos_run("msg-worker", 8192, 1, naos_msg_worker);

  // run cleaner
  naos_repeat("msg-cleaner", 1000, naos_msg_cleaner);

  // install system endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = 0xFD,
      .name = "system",
      .handle = naos_msg_process_system,
  });
}

uint8_t naos_msg_register(naos_msg_channel_t channel) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // check count
  if (naos_msg_channel_count >= NAOS_MSG_MAX_CHANNELS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // get num
  uint8_t num = naos_msg_channel_count;

  // store transport
  naos_msg_channels[naos_msg_channel_count] = channel;
  naos_msg_channel_count++;

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  return num;
}

void naos_msg_install(naos_msg_endpoint_t endpoint) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // check count
  if (naos_msg_endpoint_count >= NAOS_MSG_MAX_ENDPOINTS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // store transport
  naos_msg_endpoints[naos_msg_endpoint_count] = endpoint;
  naos_msg_endpoint_count++;

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);
}

bool naos_msg_dispatch(uint8_t channel, uint8_t* data, size_t len, void* ctx) {
#if NAOS_MSG_DEBUG
  ESP_LOGI("MSG", "incoming message:");
  ESP_LOG_BUFFER_HEX("MSG", data, len);
#endif

  // check length
  if (len < 4) {
    ESP_LOGE("MSG", "naos_msg_dispatch: message too short");
    return false;
  }

  // check version
  if (data[0] != 1) {
    ESP_LOGE("MSG", "naos_msg_dispatch: invalid version");
    return false;
  }

  // get session id
  uint16_t sid;
  memcpy(&sid, data + 1, 2);

  // get endpoint ID
  uint8_t eid = data[3];

  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // handle "begin" command
  if (eid == 0) {
    // check sid
    if (sid != 0) {
      NAOS_UNLOCK(naos_msg_mutex);
      ESP_LOGE("MSG", "naos_msg_dispatch: unexpected session ID");
      return false;
    }

    // find free session
    naos_msg_session_t* session = NULL;
    for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
      if (!naos_msg_session[i].active) {
        session = &naos_msg_session[i];
        break;
      }
    }
    if (session == NULL) {
      NAOS_UNLOCK(naos_msg_mutex);
      ESP_LOGE("MSG", "naos_msg_dispatch: no free session");
      return false;
    }

    // set active
    session->active = true;

    // assign id
    session->id = naos_msg_next_session;
    naos_msg_next_session++;

    // set channel
    session->channel = channel;

    // set context
    session->context = ctx;

    // set time
    session->last_msg = naos_millis();

    // set lock status
    session->locked = strlen(naos_get_s("device-password")) > 0;

    // prepare reply
    memcpy(data + 1, &session->id, 2);

    // release mutex
    NAOS_UNLOCK(naos_msg_mutex);

#if NAOS_MSG_DEBUG
    ESP_LOGI("MSG", "outgoing message:");
    ESP_LOG_BUFFER_HEX("MSG", data, len);
#endif

    // send reply
    if (!naos_msg_channels[channel].send(data, len, ctx)) {
      ESP_LOGE("MSG", "naos_msg_dispatch: failed to send reply");
    }

    return true;
  }

  // find session
  naos_msg_session_t* session = naos_msg_find(sid);
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    ESP_LOGE("MSG", "naos_msg_dispatch: session not found");
    return false;
  }

  // verify session
  if (session->channel != channel) {
    NAOS_UNLOCK(naos_msg_mutex);
    ESP_LOGE("MSG", "naos_msg_dispatch: session state mismatch");
    return false;
  }

  // handle "ping" command
  if (eid == 0xFE) {
    // update last message
    session->last_msg = naos_millis();

    // release mutex
    NAOS_UNLOCK(naos_msg_mutex);

    // prepare reply
    uint8_t reply[] = {1, 0, 0, 0xFE, NAOS_MSG_ACK};
    memcpy(reply + 1, &session->id, 2);

#if NAOS_MSG_DEBUG
    ESP_LOGI("MSG", "outgoing message:");
    ESP_LOG_BUFFER_HEX("MSG", reply, 5);
#endif

    // send reply
    if (!naos_msg_channels[channel].send(reply, 5, ctx)) {
      ESP_LOGE("MSG", "naos_msg_dispatch: failed to send reply");
    }

    return true;
  }

  // handle "end" command
  if (eid == 0xFF) {
    // clean up endpoints
    for (size_t i = 0; i < naos_msg_endpoint_count; i++) {
      if (naos_msg_endpoints[i].cleanup != NULL) {
        naos_msg_endpoints[i].cleanup(session->id);
      }
    }

    // clear session
    *session = (naos_msg_session_t){0};

    // release mutex
    NAOS_UNLOCK(naos_msg_mutex);

#if NAOS_MSG_DEBUG
    ESP_LOGI("MSG", "outgoing message:");
    ESP_LOG_BUFFER_HEX("MSG", data, 4);
#endif

    // send reply
    if (!naos_msg_channels[channel].send(data, 4, ctx)) {
      ESP_LOGE("MSG", "naos_msg_dispatch: failed to send reply");
    }

    return true;
  }

  // handle "query" command
  if (len == 4) {
    // update last message
    session->last_msg = naos_millis();

    // find endpoint
    bool found = false;
    for (size_t i = 0; i < naos_msg_endpoint_count; i++) {
      if (naos_msg_endpoints[i].ref == eid) {
        found = true;
        break;
      }
    }

    // release mutex
    NAOS_UNLOCK(naos_msg_mutex);

    // prepare reply
    uint8_t reply[] = {1, 0, 0, 0xFE, found ? NAOS_MSG_ACK : NAOS_MSG_UNKNOWN};
    memcpy(reply + 1, &session->id, 2);

#if NAOS_MSG_DEBUG
    ESP_LOGI("MSG", "outgoing message:");
    ESP_LOG_BUFFER_HEX("MSG", reply, 5);
#endif

    // send reply
    if (!naos_msg_channels[channel].send(reply, 5, ctx)) {
      ESP_LOGE("MSG", "naos_msg_dispatch: failed to send reply");
    }

    return true;
  }

  // TODO: Pre-check endpoint?
  //  Return error if endpoint is missing?

  // copy data
  uint8_t* copy = malloc(len - 4 + 1);
  memcpy(copy, data + 4, len - 4);
  copy[len - 4] = 0;

  // prepare message
  naos_msg_t msg = {
      .session = session->id,
      .endpoint = eid,
      .data = copy,
      .len = len - 4,
  };

  // update last
  session->last_msg = naos_millis();

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  // queue message
  naos_push(naos_msg_queue, &msg, -1);

  return true;
}

bool naos_msg_send(naos_msg_t msg) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // find session
  naos_msg_session_t* session = naos_msg_find(msg.session);
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    ESP_LOGE("MSG", "naos_msg_send: session not found");
    return false;
  }

  // update last
  session->last_msg = naos_millis();

  // get channel
  naos_msg_channel_t channel = naos_msg_channels[session->channel];

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  // check channel MTU
  if (4 + msg.len > channel.mtu(session->context)) {
    ESP_LOGE("MSG", "naos_msg_send: message too large");
    return false;
  }

  // re-frame message
  uint8_t* frame = malloc(4 + msg.len);
  frame[0] = 1;  // version
  memcpy(&frame[1], &msg.session, 2);
  frame[3] = msg.endpoint;
  memcpy(&frame[4], msg.data, msg.len);

#if NAOS_MSG_DEBUG
  ESP_LOGI("MSG", "outgoing message:");
  ESP_LOG_BUFFER_HEX("MSG", frame, 4 + msg.len);
#endif

  // send message via channel
  bool ok = channel.send(frame, 4 + msg.len, session->context);
  if (!ok) {
    ESP_LOGE("MSG", "naos_msg_send: failed to send message");
  }

  // free frame
  free(frame);

  return ok;
}

size_t naos_msg_get_mtu(uint16_t id) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // find session
  naos_msg_session_t* session = naos_msg_find(id);
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    ESP_LOGE("MSG", "naos_msg_get_mtu: session not found");
    return 0;
  }

  // get channel
  naos_msg_channel_t channel = naos_msg_channels[session->channel];

  // determine MTU
  size_t mtu = channel.mtu(session->context);

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  return mtu;
}

bool naos_msg_is_locked(uint16_t id) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // find session
  naos_msg_session_t* session = naos_msg_find(id);
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    ESP_LOGE("MSG", "naos_msg_is_locked: session not found");
    return 0;
  }

  // get lock status
  bool locked = session->locked;

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  return locked;
}
