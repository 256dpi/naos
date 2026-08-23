#include <string.h>

#include <naos.h>
#include <naos/msg.h>
#include <naos/sys.h>

#include <esp_log.h>
#include <esp_heap_caps.h>
#include <esp_websocket_client.h>
#include <esp_crt_bundle.h>

#include "system.h"
#include "utils.h"

#define NAOS_CONNECT_VERSION 0x1
#define NAOS_CONNECT_BUFFER 4096
#define NAOS_CONNECT_TIMEOUT 5000

typedef enum : uint8_t {
  NAOS_CONNECT_MSG,
} naos_connect_command_t;

typedef struct {
  uint8_t version;
  naos_connect_command_t cmd;
} naos_connect_header_t;

typedef enum {
  NAOS_CONNECT_STOPPED,
  NAOS_CONNECT_STARTING,
  NAOS_CONNECT_STARTED,
  NAOS_CONNECT_CONNECTED,
} naos_connect_state_t;

// Locking model:
// - naos_connect_mutex protects logical connection state.
// - naos_connect_client_mutex serializes client lifecycle and send access.
// - Code that needs both locks must always take naos_connect_client_mutex first
//   and naos_connect_mutex second to avoid races and lock inversion.
// - The websocket client task holds the client's internal lock while it
//   dispatches events, so sends issued from the event handler (inline message
//   dispatch may send replies) must not wait on naos_connect_client_mutex, as
//   another task may hold it while waiting for the client's internal lock
//   inside a send. Handler sends therefore skip the client mutex, which is
//   safe as the client cannot be stopped and destroyed while its task
//   dispatches an event. Messages are framed contiguously and sent with a
//   single call, so the client's internal lock alone serializes concurrent
//   sends. All sends use bounded timeouts as a last line of defense.

typedef enum {
  NAOS_CONNECT_RX_IDLE,
  NAOS_CONNECT_RX_ACTIVE,
  NAOS_CONNECT_RX_SKIP,
} naos_connect_rx_state_t;

static naos_mutex_t naos_connect_mutex;
static naos_mutex_t naos_connect_client_mutex;
static esp_websocket_client_handle_t naos_connect_client;
static naos_task_t naos_connect_dispatching = NULL;
static uint8_t naos_connect_channel = 0;
static naos_connect_state_t naos_connect_state = NAOS_CONNECT_STOPPED;

// receive re-assembly state, only accessed from the websocket client task's
// event handler, which dispatches events sequentially
static naos_connect_rx_state_t naos_connect_rx_state = NAOS_CONNECT_RX_IDLE;
static uint8_t* naos_connect_rx_buffer = NULL;
static size_t naos_connect_rx_length = 0;

static void naos_connect_handler(void *p, esp_event_base_t b, int32_t id, void *d);

static esp_websocket_client_handle_t naos_connect_client_create(const char *url, const char *token) {
  // handle scheme and select transport
  esp_websocket_transport_t transport = WEBSOCKET_TRANSPORT_OVER_TCP;
  if (strncasecmp(url, "wss://", 6) == 0) {
    transport = WEBSOCKET_TRANSPORT_OVER_SSL;
  }

  // prepare headers
  char *headers = NULL;
  if (strlen(token) > 0) {
    headers = naos_format("Authorization: %s\r\n", token);
  }

  // create client
  esp_websocket_client_config_t config = {
      .uri = url,
      .headers = headers,
      .buffer_size = NAOS_CONNECT_BUFFER,
      .transport = transport,
      .subprotocol = "naos",
      .reconnect_timeout_ms = NAOS_CONNECT_TIMEOUT,
      .network_timeout_ms = NAOS_CONNECT_TIMEOUT,
      .crt_bundle_attach = esp_crt_bundle_attach,
  };
  esp_websocket_client_handle_t client = esp_websocket_client_init(&config);
  free(headers);
  if (client == NULL) {
    return NULL;
  }

  // register events
  ESP_ERROR_CHECK(esp_websocket_register_events(client, WEBSOCKET_EVENT_ANY, naos_connect_handler, NULL));

  return client;
}

static void naos_connect_start() {
  // get settings
  const char *url = naos_get_s("connect-url");
  const char *token = naos_get_s("connect-token");

  // return if host is empty
  if (strlen(url) == 0) {
    return;
  }

  // acquire client lock
  naos_lock(naos_connect_client_mutex);

  // re-check state after taking the client lock
  naos_lock(naos_connect_mutex);
  if (naos_connect_state != NAOS_CONNECT_STOPPED) {
    naos_unlock(naos_connect_mutex);
    naos_unlock(naos_connect_client_mutex);
    return;
  }
  naos_connect_state = NAOS_CONNECT_STARTING;
  naos_unlock(naos_connect_mutex);

  // check previous client
  if (naos_connect_client != NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // create and start new client
  naos_connect_client = naos_connect_client_create(url, token);
  if (naos_connect_client != NULL) {
    esp_err_t err = esp_websocket_client_start(naos_connect_client);
    if (err != ESP_OK) {
      ESP_LOGE(NAOS_LOG_TAG, "naos_connect_start: failed to start client: %s", esp_err_to_name(err));
      ESP_ERROR_CHECK(esp_websocket_client_destroy(naos_connect_client));
      naos_connect_client = NULL;
    }
  }

  // finalize state
  naos_lock(naos_connect_mutex);
  if (naos_connect_client == NULL) {
    naos_connect_state = NAOS_CONNECT_STOPPED;
  } else if (naos_connect_state == NAOS_CONNECT_STARTING) {
    naos_connect_state = NAOS_CONNECT_STARTED;
  }
  naos_unlock(naos_connect_mutex);

  // surface invalid URL via status
  if (naos_connect_client == NULL) {
    naos_set_s("connect-status", "invalid");
  }

  // release client lock
  naos_unlock(naos_connect_client_mutex);
}

static void naos_connect_stop() {
  // serialize against start/destroy and re-check state under the same lock order as start
  naos_lock(naos_connect_client_mutex);
  naos_lock(naos_connect_mutex);
  if (naos_connect_state == NAOS_CONNECT_STOPPED) {
    naos_unlock(naos_connect_mutex);
    naos_unlock(naos_connect_client_mutex);
    return;
  }
  naos_connect_state = NAOS_CONNECT_STOPPED;
  naos_unlock(naos_connect_mutex);

  // clear status
  naos_set_s("connect-status", "");

  // stop and destroy the client
  if (naos_connect_client != NULL) {
    ESP_ERROR_CHECK(esp_websocket_client_stop(naos_connect_client));
    ESP_ERROR_CHECK(esp_websocket_client_destroy(naos_connect_client));
    naos_connect_client = NULL;
  }
  naos_unlock(naos_connect_client_mutex);
}

static void naos_connect_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_connect_configure: called");

  // stop and start
  naos_connect_stop();
  if (naos_status() >= NAOS_CONNECTED) {
    naos_connect_start();
  }
}

static void naos_connect_manage(naos_status_t status) {
  // get network status
  bool connected = status >= NAOS_CONNECTED;

  // get state
  naos_lock(naos_connect_mutex);
  naos_connect_state_t state = naos_connect_state;
  naos_unlock(naos_connect_mutex);

  // handle status
  if (connected && state == NAOS_CONNECT_STOPPED) {
    naos_connect_start();
  } else if (!connected && state != NAOS_CONNECT_STOPPED) {
    naos_connect_stop();
  }
}

static void naos_connect_handler(void *p, esp_event_base_t b, int32_t id, void *d) {
  // get data
  esp_websocket_event_data_t *data = (esp_websocket_event_data_t *)d;

  // handle event
  switch (id) {
    case WEBSOCKET_EVENT_CONNECTED:
      // log event
      ESP_LOGI(NAOS_LOG_TAG, "naos_connect_handler: connected");

      // check and set flag
      naos_lock(naos_connect_mutex);
      if (naos_connect_state == NAOS_CONNECT_STOPPED) {
        naos_unlock(naos_connect_mutex);
        break;
      }
      naos_connect_state = NAOS_CONNECT_CONNECTED;
      naos_unlock(naos_connect_mutex);

      // reset receive state
      naos_connect_rx_state = NAOS_CONNECT_RX_IDLE;
      naos_connect_rx_length = 0;

      // set status
      naos_set_s("connect-status", "connected");

      break;

    case WEBSOCKET_EVENT_DISCONNECTED:
      // log event
      ESP_LOGI(NAOS_LOG_TAG, "naos_connect_handler: disconnected");

      // check and clear flag
      naos_lock(naos_connect_mutex);
      if (naos_connect_state == NAOS_CONNECT_STOPPED) {
        naos_unlock(naos_connect_mutex);
        break;
      }
      naos_connect_state = NAOS_CONNECT_STARTED;
      naos_unlock(naos_connect_mutex);

      // set status
      naos_set_s("connect-status", "disconnected");

      break;

    case WEBSOCKET_EVENT_DATA:
      // ignore stale events
      naos_lock(naos_connect_mutex);
      if (naos_connect_state == NAOS_CONNECT_STOPPED) {
        naos_unlock(naos_connect_mutex);
        break;
      }
      naos_unlock(naos_connect_mutex);

      // ignore interleaved control frames (close, ping, pong)
      if (data->op_code == 0x8 || data->op_code == 0x9 || data->op_code == 0xA) {
        break;
      }

      // messages may span multiple websocket frames (continuation frames with
      // opcode 0x0) and frame payloads may span multiple events (partial
      // transport reads), so all data is re-assembled into a buffer and
      // dispatched once the last chunk of the final frame is received

      // handle the first chunk of a message's first frame
      if (data->op_code != 0x0 && data->payload_offset == 0) {
        // drop unfinished previous message
        if (naos_connect_rx_state == NAOS_CONNECT_RX_ACTIVE) {
          ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: dropped incomplete message");
        }

        // begin new message
        naos_connect_rx_state = NAOS_CONNECT_RX_ACTIVE;
        naos_connect_rx_length = 0;

        // skip non-binary messages; only binary frames carry NAOS data
        if (data->op_code != 0x2) {
          ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: skipped unexpected opcode: 0x%x", data->op_code);
          naos_connect_rx_state = NAOS_CONNECT_RX_SKIP;
        }
      } else if (naos_connect_rx_state == NAOS_CONNECT_RX_IDLE) {
        // skip continuations of a message whose beginning was never received
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: skipped stray continuation");
        naos_connect_rx_state = NAOS_CONNECT_RX_SKIP;
      }

      // append chunk, skipping messages exceeding the buffer
      if (naos_connect_rx_state == NAOS_CONNECT_RX_ACTIVE && data->data_len > 0) {
        if (naos_connect_rx_length + (size_t)data->data_len > NAOS_CONNECT_BUFFER) {
          ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: skipped too long message");
          naos_connect_rx_state = NAOS_CONNECT_RX_SKIP;
          naos_connect_rx_length = 0;
        } else {
          memcpy(naos_connect_rx_buffer + naos_connect_rx_length, data->data_ptr, data->data_len);
          naos_connect_rx_length += (size_t)data->data_len;
        }
      }

      // await remaining chunks of this frame or further continuation frames
      if (data->payload_offset + data->data_len < data->payload_len || !data->fin) {
        break;
      }

      // take completed message and reset receive state
      size_t length = naos_connect_rx_length;
      bool skipped = naos_connect_rx_state == NAOS_CONNECT_RX_SKIP;
      naos_connect_rx_state = NAOS_CONNECT_RX_IDLE;
      naos_connect_rx_length = 0;
      if (skipped) {
        break;
      }

      // require a full header
      if (length < sizeof(naos_connect_header_t)) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: ignored short message");
        break;
      }

      // get header
      naos_connect_header_t *header = (naos_connect_header_t *)naos_connect_rx_buffer;

      // check version
      if (header->version != NAOS_CONNECT_VERSION) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: invalid version");
        break;
      }

      // check command
      if (header->cmd != NAOS_CONNECT_MSG) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: invalid command");
        break;
      }

      // dispatch message, marking the dispatching task so triggered sends can
      // skip the client mutex (see locking model above)
      naos_connect_dispatching = naos_current();
      naos_msg_dispatch(naos_connect_channel, naos_connect_rx_buffer + sizeof(naos_connect_header_t),
                        length - sizeof(naos_connect_header_t), NULL);
      naos_connect_dispatching = NULL;

      break;
  }
}

static bool naos_connect_send(const uint8_t *data, size_t len, void *ctx) {
  // validate payload length against the websocket frame budget
  if (len > NAOS_CONNECT_BUFFER - sizeof(naos_connect_header_t)) {
    return false;
  }

  // require an active connection
  naos_lock(naos_connect_mutex);
  if (naos_connect_state != NAOS_CONNECT_CONNECTED) {
    naos_unlock(naos_connect_mutex);
    return false;
  }
  naos_unlock(naos_connect_mutex);

  // frame message contiguously to allow an atomic single-call send
  size_t total = sizeof(naos_connect_header_t) + len;
  uint8_t *frame = malloc(total);
  if (frame == NULL) {
    return false;
  }
  naos_connect_header_t *header = (naos_connect_header_t *)frame;
  header->version = NAOS_CONNECT_VERSION;
  header->cmd = NAOS_CONNECT_MSG;
  memcpy(frame + sizeof(naos_connect_header_t), data, len);

  // skip the client mutex when sending from the event handler
  // (see locking model above)
  bool inline_send = naos_connect_dispatching == naos_current();

  // serialize client access against stop/destroy for cross-task sends
  if (!inline_send) {
    naos_lock(naos_connect_client_mutex);
  }
  int ret = -1;
  if (naos_connect_client != NULL) {
    ret = esp_websocket_client_send_bin(naos_connect_client, (char *)frame, (int)total,
                                        pdMS_TO_TICKS(NAOS_CONNECT_TIMEOUT));
  }
  if (!inline_send) {
    naos_unlock(naos_connect_client_mutex);
  }

  // free frame
  free(frame);

  return ret >= 0;
}

static uint16_t naos_connect_mtu() {
  // calculate MTU
  return NAOS_CONNECT_BUFFER - sizeof(naos_connect_header_t);
}

static naos_param_t naos_connect_params[] = {
    {.name = "connect-url", .type = NAOS_STRING},
    {.name = "connect-token", .type = NAOS_STRING},
    {.name = "connect-configure", .type = NAOS_ACTION, .func_a = naos_connect_configure},
    {.name = "connect-status", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_LOCKED},
};

void naos_connect_init() {
  // initialize mutexes
  naos_connect_mutex = naos_mutex();
  naos_connect_client_mutex = naos_mutex();

  // allocate receive buffer
  naos_connect_rx_buffer = naos_alloc(NAOS_CONNECT_BUFFER);

  // register parameters
  for (size_t i = 0; i < NAOS_COUNT(naos_connect_params); i++) {
    naos_register(&naos_connect_params[i]);
  }

  // register the connect channel as trusted: the device authenticates against
  // the configured endpoint via token, so incoming sessions start unlocked
  naos_connect_channel = naos_msg_register((naos_msg_channel_t){
      .name = "naos-conn",
      .mtu = naos_connect_mtu,
      .send = naos_connect_send,
      .trusted = true,
  });

  // handle status
  naos_system_subscribe(naos_connect_manage);
}
