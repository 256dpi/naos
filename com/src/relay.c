#include <naos/relay.h>
#include <naos/msg.h>

#define NAOS_RELAY_ENDPOINT 0x4
#define NAOS_RELAY_LINKS 8

typedef enum {
  NAOS_RELAY_CMD_SCAN,
  NAOS_RELAY_CMD_LINK,
  NAOS_RELAY_CMD_SEND,
} naos_relay_cmd_t;

typedef struct {
  uint16_t session;
  uint8_t device;
} naos_relay_link_t;

static naos_relay_host_t naos_relay_host;
static naos_relay_device_t naos_relay_device;
static uint8_t naos_relay_channel;
static naos_relay_link_t naos_relay_links[NAOS_RELAY_LINKS] = {0};

static naos_msg_reply_t naos_relay_handle_scan(naos_msg_t msg) {
  // empty message

  // check length
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // scan devices
  uint64_t devices = naos_relay_host.scan();

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_RELAY_ENDPOINT,
      .data = (uint8_t *)&devices,
      .len = sizeof(devices),
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_relay_handle_link(naos_msg_t msg) {
  // message structure:
  // NUM (1)

  // check length
  if (msg.len != 1) {
    return NAOS_MSG_INVALID;
  }

  // check existing links
  for (size_t i = 0; i < NAOS_RELAY_LINKS; i++) {
    if (naos_relay_links[i].session == msg.session) {
      return NAOS_MSG_ERROR;
    }
  }

  // get number
  uint8_t num = msg.data[0];

  // create link
  bool ok = false;
  for (size_t i = 0; i < NAOS_RELAY_LINKS; i++) {
    if (naos_relay_links[i].session == 0) {
      naos_relay_links[i] = (naos_relay_link_t){
          .session = msg.session,
          .device = num,
      };
      ok = true;
      break;
    }
  }
  if (!ok) {
    return NAOS_MSG_ERROR;
  }

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_relay_handle_send(naos_msg_t msg) {
  // message structure
  // NUM (1) | DATA (*)

  // check length
  if (msg.len < 5) {
    return NAOS_MSG_INVALID;
  }

  // pluck number
  uint8_t num = msg.data[0];
  msg.data++;
  msg.len--;

  // check links
  bool ok = false;
  for (size_t i = 0; i < NAOS_RELAY_LINKS; i++) {
    naos_relay_link_t *link = &naos_relay_links[i];
    if (link->session == msg.session && link->device == num) {
      ok = true;
    }
  }
  if (!ok) {
    return NAOS_MSG_ERROR;
  }

  // prepare meta
  naos_relay_meta_t meta = {
      .mtu = naos_msg_get_mtu(msg.session),
  };

  // send message downstream
  if (!naos_relay_host.send(num, msg.data, msg.len, meta)) {
    return NAOS_MSG_ERROR;
  }

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_relay_handle(naos_msg_t msg) {
  // message structure
  // CMD (1) | *

  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // we do not check the lock status

  // pluck command
  naos_relay_cmd_t cmd = msg.data[0];
  msg.data++;
  msg.len--;

  // handle command
  naos_msg_reply_t reply;
  switch (cmd) {
    case NAOS_RELAY_CMD_SCAN:
      reply = naos_relay_handle_scan(msg);
      break;
    case NAOS_RELAY_CMD_LINK:
      reply = naos_relay_handle_link(msg);
      break;
    case NAOS_RELAY_CMD_SEND:
      reply = naos_relay_handle_send(msg);
      break;
    default:
      reply = NAOS_MSG_UNKNOWN;
  }

  return reply;
}

static void naos_relay_cleanup(uint16_t session) {
  // clear links
  for (size_t i = 0; i < NAOS_RELAY_LINKS; i++) {
    if (naos_relay_links[i].session == session) {
      naos_relay_links[i] = (naos_relay_link_t){0};
    }
  }
}

static bool naos_relay_device_send(const uint8_t *data, size_t len, void *ctx) {
  // CONTEXT MAY BE INVALID!

  // send message upstream
  return naos_relay_device.send(data, len);
}

static uint16_t naos_relay_device_mtu(void *ctx) {
  // get meta
  naos_relay_meta_t *meta = ctx;

  // determine MTU
  uint16_t mtu = meta->mtu;
  if (mtu > naos_relay_device.mtu) {
    mtu = naos_relay_device.mtu;
  }

  return mtu - 6;
}

void naos_relay_host_init(naos_relay_host_t config) {
  // store config
  naos_relay_host = config;

  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_RELAY_ENDPOINT,
      .name = "relay",
      .handle = naos_relay_handle,
      .cleanup = naos_relay_cleanup,
  });
}

void naos_relay_device_init(naos_relay_device_t config) {
  // store config
  naos_relay_device = config;

  // register channel
  naos_relay_channel = naos_msg_register((naos_msg_channel_t){
      .name = "relay",
      .mtu = naos_relay_device_mtu,
      .send = naos_relay_device_send,
  });
}

void naos_relay_host_process(uint8_t num, uint8_t *data, size_t len) {
  // prepare message
  naos_msg_t msg = {
      .session = 0,
      .endpoint = NAOS_RELAY_ENDPOINT,
      .data = data,
      .len = len,
  };

  // iterate links
  for (size_t i = 0; i < NAOS_RELAY_LINKS; i++) {
    naos_relay_link_t *link = &naos_relay_links[i];
    if (link->session > 0 && link->device == num) {
      msg.session = link->session;
      naos_msg_send(msg);
    }
  }
}

void naos_relay_device_process(uint8_t *data, size_t len, naos_relay_meta_t meta) {
  // dispatch message
  naos_msg_dispatch(naos_relay_channel, data, len, &meta);
}
