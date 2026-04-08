#include <naos.h>
#include <naos/msg.h>
#include <naos/sys.h>

#include <esp_log.h>
#include <esp_websocket_client.h>
#include <esp_crt_bundle.h>

#include "system.h"
#include "utils.h"

#define NAOS_CONNECT_VERSION 0x1
#define NAOS_CONNECT_BUFFER 4096

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

static naos_mutex_t naos_connect_mutex;
static naos_mutex_t naos_connect_client_mutex;
static esp_websocket_client_handle_t naos_connect_client;
static uint8_t naos_connect_channel = 0;
static naos_connect_state_t naos_connect_state = NAOS_CONNECT_STOPPED;

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
      .reconnect_timeout_ms = 5000,
      .network_timeout_ms = 5000,
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

  // serialize client lifecycle
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

  if (naos_connect_client != NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }
  naos_connect_client = naos_connect_client_create(url, token);
  if (naos_connect_client == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }
  ESP_ERROR_CHECK(esp_websocket_client_start(naos_connect_client));

  naos_lock(naos_connect_mutex);
  if (naos_connect_state == NAOS_CONNECT_STARTING) {
    naos_connect_state = NAOS_CONNECT_STARTED;
  }
  naos_unlock(naos_connect_mutex);
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

      // ignore non-binary messages
      if (data->op_code != 0x2) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: ignored non-binary message");
        break;
      }

      // ignore chunked messages
      if (data->payload_offset > 0 || data->payload_len > data->data_len) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: ignored chunked message");
        break;
      }

      // require a full header
      if (data->data_len < sizeof(naos_connect_header_t)) {
        ESP_LOGE(NAOS_LOG_TAG, "naos_connect_handler: ignored short message");
        break;
      }

      // get header
      naos_connect_header_t *header = (naos_connect_header_t *)data->data_ptr;

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

      // get buffer and length
      uint8_t *buffer = (uint8_t *)(data->data_ptr) + sizeof(naos_connect_header_t);
      size_t length = data->data_len - sizeof(naos_connect_header_t);

      // dispatch message
      naos_msg_dispatch(naos_connect_channel, buffer, length, NULL);

      break;
  }
}

static bool naos_connect_send(const uint8_t *data, size_t len, void *ctx) {
  // prepare header
  naos_connect_header_t header = {
      .version = NAOS_CONNECT_VERSION,
      .cmd = NAOS_CONNECT_MSG,
  };

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

  // serialize client access against stop/destroy and other sends
  naos_lock(naos_connect_client_mutex);
  esp_websocket_client_handle_t client = naos_connect_client;
  if (client == NULL) {
    naos_unlock(naos_connect_client_mutex);
    return false;
  }
  int r1 = esp_websocket_client_send_bin_partial(client, (char *)&header, sizeof(naos_connect_header_t), portMAX_DELAY);
  int r2 = esp_websocket_client_send_cont_msg(client, (char *)data, (int)len, portMAX_DELAY);
  int r3 = esp_websocket_client_send_fin(client, portMAX_DELAY);
  naos_unlock(naos_connect_client_mutex);

  return r1 >= 0 && r2 >= 0 && r3 >= 0;
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

  // register parameters
  for (size_t i = 0; i < NAOS_COUNT(naos_connect_params); i++) {
    naos_register(&naos_connect_params[i]);
  }

  // register the connect channel
  naos_connect_channel = naos_msg_register((naos_msg_channel_t){
      .name = "naos-conn",
      .mtu = naos_connect_mtu,
      .send = naos_connect_send,
  });

  // handle status
  naos_system_subscribe(naos_connect_manage);
}
