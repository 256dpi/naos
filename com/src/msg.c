#include <naos/sys.h>
#include <string.h>

#include "msg.h"

#define NAOS_MSG_MAX_CHANNELS 8
#define NAOS_MSG_MAX_ENDPOINTS 32
#define NAOS_MSG_MAX_SESSIONS 64

typedef struct {
  bool active;
  uint16_t id;
  size_t channel;
  void* context;
  int64_t last_msg;
} naos_msg_session_t;

static naos_mutex_t naos_msg_mutex;
static naos_queue_t naos_msg_queue;
static naos_msg_channel_t naos_msg_channels[NAOS_MSG_MAX_CHANNELS] = {0};
static size_t naos_msg_channel_count = 0;
static naos_msg_endpoint_t naos_msg_endpoints[NAOS_MSG_MAX_ENDPOINTS] = {0};
static size_t naos_msg_endpoint_count = 0;
static naos_msg_session_t naos_msg_session[NAOS_MSG_MAX_SESSIONS] = {0};
static uint16_t naos_msg_next_session = 1;

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
    naos_msg_err_t err = endpoint->handle(msg);

    // send error
    if (err != NAOS_MSG_OK) {
      naos_msg_endpoint_send((naos_msg_t){
          .session = msg.session,
          .endpoint = 0xFE,
          .data = (uint8_t*)&err,
          .len = 1,
      });
    }

    // free data
    free(msg.data);
  }
}

void naos_msg_cleaner() {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // get current time
  int64_t now = naos_millis();

  // iterate over sessions
  for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
    // get session
    naos_msg_session_t* session = &naos_msg_session[i];

    // skip if inactive and young sessions
    if (!session->active || now - session->last_msg < 30000) {
      continue;
    }

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

void naos_msg_init() {
  // create mutex
  naos_msg_mutex = naos_mutex();

  // create queue
  naos_msg_queue = naos_queue(NAOS_MSG_MAX_SESSIONS, sizeof(naos_msg_t));

  // run worker
  naos_run("msg-worker", 8192, 1, naos_msg_worker);

  // run cleaner
  naos_repeat("msg-cleaner", 1000, naos_msg_cleaner);
}

uint8_t naos_msg_channel_register(naos_msg_channel_t channel) {
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

void naos_msg_endpoint_register(naos_msg_endpoint_t endpoint) {
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

bool naos_msg_channel_dispatch(uint8_t channel, uint8_t* data, size_t len, void* ctx) {
  // check length
  if (len < 4) {
    return false;
  }

  // check version
  if (data[0] != 1) {
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

    // prepare reply
    memcpy(data + 1, &session->id, 2);

    // release mutex
    NAOS_UNLOCK(naos_msg_mutex);

    // send reply
    naos_msg_channels[channel].send(data, len, ctx);

    return true;
  }

  // find session
  naos_msg_session_t* session = NULL;
  for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
    if (naos_msg_session[i].id == sid) {
      session = &naos_msg_session[i];
      break;
    }
  }
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    return false;
  }

  // verify session
  if (!session->active || session->channel != channel) {
    NAOS_UNLOCK(naos_msg_mutex);
    return false;
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

    // send reply
    naos_msg_channels[channel].send(data, 4, ctx);

    return true;
  }

  // copy data
  uint8_t* copy = NULL;
  if (len > 4) {
    copy = malloc(len - 4 + 1);
    memcpy(copy, data + 4, len - 4);
    copy[len - 4] = 0;
  }

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

bool naos_msg_endpoint_send(naos_msg_t msg) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // find session
  naos_msg_session_t* session = NULL;
  for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
    if (naos_msg_session[i].id == msg.session) {
      session = &naos_msg_session[i];
      break;
    }
  }
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    return false;
  }

  // update last
  session->last_msg = naos_millis();

  // get channel
  naos_msg_channel_t channel = naos_msg_channels[session->channel];

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  // re-frame message
  uint8_t* frame = malloc(4 + msg.len);
  frame[0] = 1;  // version
  memcpy(&frame[1], &msg.session, 2);
  frame[3] = msg.endpoint;
  memcpy(&frame[4], msg.data, msg.len);

  // send message via channel
  bool ok = channel.send(frame, 4 + msg.len, session->context);

  // free frame
  free(frame);

  return ok;
}

size_t naos_msg_session_mtu(uint16_t id) {
  // acquire mutex
  NAOS_LOCK(naos_msg_mutex);

  // find session
  naos_msg_session_t* session = NULL;
  for (size_t i = 0; i < NAOS_MSG_MAX_SESSIONS; i++) {
    if (naos_msg_session[i].id == id) {
      session = &naos_msg_session[i];
      break;
    }
  }
  if (session == NULL) {
    NAOS_UNLOCK(naos_msg_mutex);
    return 0;
  }

  // get channel
  naos_msg_channel_t channel = naos_msg_channels[session->channel];

  // release mutex
  NAOS_UNLOCK(naos_msg_mutex);

  return channel.mtu;
}
