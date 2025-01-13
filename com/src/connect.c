#include <naos.h>
#include <naos/msg.h>
#include <naos/sys.h>

#include <string.h>
#include <esp_websocket_client.h>

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

static naos_mutex_t naos_connect_mutex;
static esp_websocket_client_handle_t naos_connect_client;
static uint8_t naos_connect_channel = 0;
static bool naos_connect_started = false;
static bool naos_connect_connected = false;

static void naos_connect_start() {
  // get settings
  const char *url = naos_get_s("connect-url");
  const char *token = naos_get_s("connect-token");

  // return if host is empty
  if (strlen(url) == 0) {
    return;
  }

  // set flag
  NAOS_LOCK(naos_connect_mutex);
  naos_connect_started = true;
  NAOS_UNLOCK(naos_connect_mutex);

  // prepare headers
  char headers[256];
  snprintf(headers, sizeof(headers), "Authorization: %s\r\n", token);

  // configure the client
  ESP_ERROR_CHECK(esp_websocket_client_set_uri(naos_connect_client, url));
  // ESP_ERROR_CHECK(esp_websocket_client_set_headers(naos_connect_client, headers));

  // start the MQTT client
  ESP_ERROR_CHECK(esp_websocket_client_start(naos_connect_client));
}

static void naos_connect_stop() {
  // stop the MQTT client
  ESP_ERROR_CHECK(esp_websocket_client_stop(naos_connect_client));

  // set flags
  NAOS_LOCK(naos_connect_mutex);
  naos_connect_started = false;
  naos_connect_connected = false;
  NAOS_UNLOCK(naos_connect_mutex);
}

static void naos_connect_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_connect_configure");

  // get started
  NAOS_LOCK(naos_connect_mutex);
  bool started = naos_connect_started;
  NAOS_UNLOCK(naos_connect_mutex);

  // restart if started
  if (started) {
    naos_connect_stop();
    naos_connect_start();
  }
}

static void naos_connect_manage(naos_status_t status) {
  // get network status
  bool connected = status >= NAOS_CONNECTED;

  // get started
  NAOS_LOCK(naos_connect_mutex);
  bool started = naos_connect_started;
  NAOS_UNLOCK(naos_connect_mutex);

  // handle status
  if (connected && !started) {
    naos_connect_start();
  } else if (!connected && started) {
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

      // set flag
      NAOS_LOCK(naos_connect_mutex);
      naos_connect_connected = true;
      NAOS_UNLOCK(naos_connect_mutex);

      // set status
      naos_set_s("connect-status", "connected");

      break;

    case WEBSOCKET_EVENT_DISCONNECTED:
      // log event
      ESP_LOGI(NAOS_LOG_TAG, "naos_connect_handler: disconnected");

      // set flag
      NAOS_LOCK(naos_connect_mutex);
      naos_connect_connected = false;
      NAOS_UNLOCK(naos_connect_mutex);

      // set status
      naos_set_s("connect-status", "disconnected");

      break;

    case WEBSOCKET_EVENT_DATA:
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
  // get status
  NAOS_LOCK(naos_connect_mutex);
  bool connected = naos_connect_connected;
  NAOS_UNLOCK(naos_connect_mutex);

  // return if not connected
  if (!connected) {
    return false;
  }

  // send message
  naos_connect_header_t header = {
      .version = NAOS_CONNECT_VERSION,
      .cmd = NAOS_CONNECT_MSG,
  };
  int r1 = esp_websocket_client_send_bin_partial(naos_connect_client, (char *)&header, sizeof(naos_connect_header_t),
                                                 portMAX_DELAY);
  int r2 = esp_websocket_client_send_cont_msg(naos_connect_client, (char *)data, (int)len, portMAX_DELAY);
  int r3 = esp_websocket_client_send_fin(naos_connect_client, portMAX_DELAY);

  return r1 >= 0 && r2 >= 0 && r3 >= 0;
}

static uint16_t naos_connect_mtu() { return NAOS_CONNECT_BUFFER; }

static naos_param_t naos_connect_params[] = {
    {.name = "connect-url", .type = NAOS_STRING},
    {.name = "connect-token", .type = NAOS_STRING},
    {.name = "connect-configure", .type = NAOS_ACTION, .func_a = naos_connect_configure},
    {.name = "connect-status", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_LOCKED},
};

void naos_connect_init() {
  // initialize mutex
  naos_connect_mutex = naos_mutex();

  // register parameters
  for (size_t i = 0; i < NAOS_NUM_PARAMS(naos_connect_params); i++) {
    naos_register(&naos_connect_params[i]);
  }

  // initialize client
  esp_websocket_client_config_t config = {
      .buffer_size = NAOS_CONNECT_BUFFER,
      .transport = WEBSOCKET_TRANSPORT_OVER_TCP,
      .subprotocol = "naos",
      .reconnect_timeout_ms = 5000,
      .network_timeout_ms = 5000,
  };
  naos_connect_client = esp_websocket_client_init(&config);
  if (naos_connect_client == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // register websocket events
  ESP_ERROR_CHECK(esp_websocket_register_events(naos_connect_client, WEBSOCKET_EVENT_ANY, naos_connect_handler, NULL));

  // register the connect channel
  naos_connect_channel = naos_msg_register((naos_msg_channel_t){
      .name = "naos-conn",
      .mtu = naos_connect_mtu,
      .send = naos_connect_send,
  });

  // handle status
  naos_system_subscribe(naos_connect_manage);
}
