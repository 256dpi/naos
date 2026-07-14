#include <naos/msg.h>
#include <naos/http.h>

#include <esp_http_server.h>
#include <freertos/FreeRTOS.h>

#define NAOS_HTTP_MAX_CONNS 7
#define NAOS_HTTP_MAX_FILES 8

// Max frames queued for async send. Kept below the httpd UDP control mailbox
// depth (CONFIG_LWIP_UDP_RECVMBOX_SIZE, default 6) so httpd_queue_work can never
// overflow it and silently drop (and leak) a queued frame.
#define NAOS_HTTP_MAX_INFLIGHT 4

typedef struct {
  int fd;
} naos_http_ctx_t;

typedef struct {
  const char *path;
  const char *type;
  const char *encoding;
  const uint8_t *content;
  size_t length;
} naos_http_file_t;

typedef struct {
  uint8_t *payload;
  size_t len;
  naos_http_ctx_t *ctx;
} naos_http_msg_t;

static httpd_handle_t naos_http_handle = {0};
static naos_http_file_t naos_http_files[NAOS_HTTP_MAX_FILES] = {0};
static size_t naos_http_file_count = 0;
static uint8_t naos_http_channel = 0;
static portMUX_TYPE naos_http_inflight_mux = portMUX_INITIALIZER_UNLOCKED;
static int naos_http_inflight = 0;  // frames queued for async send, not yet freed

static esp_err_t naos_http_socket(httpd_req_t *conn) {
  // get context
  naos_http_ctx_t *ctx = conn->sess_ctx;

  // update fd
  ctx->fd = httpd_req_to_sockfd(conn);

  // prepare request frame
  httpd_ws_frame_t req = {
      .type = HTTPD_WS_TYPE_BINARY,
  };

  // read request frame length
  esp_err_t err = httpd_ws_recv_frame(conn, &req, 0);
  if (err != ESP_OK) {
    return err;
  }

  // ignore invalid frames
  if (req.len < 4) {
    return ESP_FAIL;
  }

  // allocate payload
  req.payload = malloc(req.len);
  if (req.payload == NULL) {
    return ESP_ERR_NO_MEM;
  }

  // read frame
  err = httpd_ws_recv_frame(conn, &req, req.len);
  if (err != ESP_OK) {
    free(req.payload);
    return err;
  }

  // handle message
  bool ok = naos_msg_dispatch(naos_http_channel, req.payload, req.len, ctx);

  // free request payload
  free(req.payload);

  return ok ? ESP_OK : ESP_FAIL;
}

static esp_err_t naos_http_request(httpd_req_t *req) {
  // handle socket messages immediately
  if (req->method != HTTP_GET) {
    return naos_http_socket(req);
  }

  // check if websocket
  char upgrade[32] = {0};
  httpd_req_get_hdr_value_str(req, "Upgrade", upgrade, sizeof(upgrade));
  bool is_ws = strncmp(upgrade, "websocket", sizeof(upgrade)) == 0;

  // handle initial websocket request
  if (is_ws) {
    // prepare context
    naos_http_ctx_t *ctx = malloc(sizeof(naos_http_ctx_t));
    *ctx = (naos_http_ctx_t){
        .fd = httpd_req_to_sockfd(req),
    };

    // set context
    req->sess_ctx = ctx;
    req->free_ctx = free;

    return ESP_OK;
  }

  /* handle non websocket requests */

  // set response header
  esp_err_t err = httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  if (err != ESP_OK) {
    return err;
  }

  // get len length
  size_t len = strlen(req->uri);
  for (size_t i = 0; i < len; i++) {
    if (req->uri[i] == '?') {
      len = i;
      break;
    }
  }

  // check files
  for (size_t i = 0; i < naos_http_file_count; i++) {
    // get file
    naos_http_file_t *file = &naos_http_files[i];

    // check path
    if (file->path != NULL && (strlen(file->path) != len || strncmp(req->uri, file->path, len) != 0)) {
      continue;
    }

    // set content type
    err = httpd_resp_set_type(req, file->type);
    if (err != ESP_OK) {
      return err;
    }

    // set content encoding if available
    if (file->encoding != NULL) {
      err = httpd_resp_set_hdr(req, "Content-Encoding", file->encoding);
      if (err != ESP_OK) {
        return err;
      }
    }

    // send response
    err = httpd_resp_send(req, (char *)file->content, (ssize_t)file->length);
    if (err != ESP_OK) {
      return err;
    }

    return ESP_OK;
  }

  // send 404 if not available
  err = httpd_resp_send_404(req);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static void naos_http_send_frame(void *arg) {
  // get message
  naos_http_msg_t *msg = arg;

  // prepare frame
  httpd_ws_frame_t frame = {
      .type = HTTPD_WS_TYPE_BINARY,
      .payload = msg->payload,
      .len = msg->len,
  };

  // send frame
  ESP_ERROR_CHECK_WITHOUT_ABORT(httpd_ws_send_frame_async(naos_http_handle, msg->ctx->fd, &frame));

  // release in-flight slot
  portENTER_CRITICAL(&naos_http_inflight_mux);
  naos_http_inflight--;
  portEXIT_CRITICAL(&naos_http_inflight_mux);

  // free message
  free(msg);
}

static uint16_t naos_http_msg_mtu() { return 4096; }

static bool naos_http_msg_send(const uint8_t *data, size_t len, void *ctx) {
  // prepare message
  naos_http_msg_t *msg = malloc(sizeof(naos_http_msg_t) + len);
  if (msg == NULL) {
    return false;
  }
  msg->payload = (void *)msg + sizeof(naos_http_msg_t);
  msg->len = len;
  msg->ctx = ctx;

  // check if context is still valid
  if (httpd_ws_get_fd_info(naos_http_handle, msg->ctx->fd) != HTTPD_WS_CLIENT_WEBSOCKET) {
    free(msg);
    return false;
  }

  // reserve an in-flight slot; drop the frame (but keep the session alive) when
  // the async send queue is saturated, bounding memory under a congested link.
  portENTER_CRITICAL(&naos_http_inflight_mux);
  bool full = naos_http_inflight >= NAOS_HTTP_MAX_INFLIGHT;
  if (!full) {
    naos_http_inflight++;
  }
  portEXIT_CRITICAL(&naos_http_inflight_mux);
  if (full) {
    free(msg);
    return true;
  }

  // copy payload
  memcpy(msg->payload, data, len);

  // queue function; on failure release the slot instead of aborting the device
  if (httpd_queue_work(naos_http_handle, naos_http_send_frame, msg) != ESP_OK) {
    portENTER_CRITICAL(&naos_http_inflight_mux);
    naos_http_inflight--;
    portEXIT_CRITICAL(&naos_http_inflight_mux);
    free(msg);
    return false;
  }

  return true;
}

static httpd_uri_t naos_http_route = {
    .uri = "*",
    .method = HTTP_GET,
    .handler = naos_http_request,
    .is_websocket = true,
    .supported_subprotocol = "naos",
};

void naos_http_init(naos_http_config_t config) {
  // prepare config
  httpd_config_t httpd_conf = HTTPD_DEFAULT_CONFIG();
  httpd_conf.max_open_sockets = NAOS_HTTP_MAX_CONNS;
  httpd_conf.uri_match_fn = httpd_uri_match_wildcard;
  httpd_conf.core_id = config.core;
  httpd_conf.lru_purge_enable = true;

  // enable TCP keep-alive so a client that vanishes without a clean close (e.g. a
  // reconnect that just drops the old WebSocket) is detected and its socket +
  // lwIP buffers reclaimed, instead of lingering until an LRU purge. A live idle
  // link stays up on the app's periodic keep-alive; only a truly silent peer is
  // probed. Reaped after idle + count*interval ~= 20 s of silence. Opt out via
  // config.no_keep_alive to restore the previous always-persist behaviour.
  if (!config.no_keep_alive) {
    httpd_conf.keep_alive_enable = true;
    httpd_conf.keep_alive_idle = 5;      // seconds of silence before the first probe
    httpd_conf.keep_alive_interval = 5;  // seconds between probes
    httpd_conf.keep_alive_count = 3;     // unanswered probes before the socket is dropped
  }

  // start server
  ESP_ERROR_CHECK(httpd_start(&naos_http_handle, &httpd_conf));

  // register handler
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route));

  // register channel
  naos_http_channel = naos_msg_register((naos_msg_channel_t){
      .name = "http",
      .mtu = naos_http_msg_mtu,
      .send = naos_http_msg_send,
  });
}

void naos_http_serve_str(const char *path, const char *type, const char *content) {
  naos_http_serve_bin(path, type, NULL, (uint8_t *)content, strlen(content));
}

void naos_http_serve_bin(const char *path, const char *type, const char *encoding, const uint8_t *content,
                         size_t length) {
  // check count
  if (naos_http_file_count >= NAOS_HTTP_MAX_FILES) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // prepare files
  naos_http_file_t file = {
      .path = path,
      .type = type,
      .encoding = encoding,
      .content = content,
      .length = length,
  };

  // store file
  naos_http_files[naos_http_file_count] = file;
  naos_http_file_count++;
}
