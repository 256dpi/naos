#include <naos/msg.h>
#include <naos/http.h>

#include <esp_http_server.h>

#include "utils.h"

#define NAOS_HTTP_MAX_CONNS 7
#define NAOS_HTTP_MAX_FILES 8

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

  // read request frame
  if (req.len) {
    // allocate payload
    req.payload = malloc(req.len + 1);
    if (req.payload == NULL) {
      return ESP_ERR_NO_MEM;
    }

    // read frame
    err = httpd_ws_recv_frame(conn, &req, req.len);
    if (err != ESP_OK) {
      free(req.payload);
      return err;
    }

    // terminate payload
    req.payload[req.len] = 0;
  }

  // handle message
  if (strncmp((char *)req.payload, "msg#", 4) == 0) {
    naos_msg_dispatch(naos_http_channel, req.payload + 4, req.len - 4, ctx);
  }

  // free request payload
  free(req.payload);

  return ESP_OK;
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
    if (strncmp(req->uri, file->path, len) != 0) {
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

  // free message
  free(msg);
}

static size_t naos_http_msg_mtu() { return 4096; }

static bool naos_http_msg_send(const uint8_t *data, size_t len, void *ctx) {
  // prepare message
  naos_http_msg_t *msg = malloc(sizeof(naos_http_msg_t) + 4 + len);
  msg->payload = (void *)msg + sizeof(naos_http_msg_t);
  msg->len = 4 + len;
  msg->ctx = ctx;

  // prepare payload
  memcpy(msg->payload, "msg#", 4);
  memcpy(msg->payload + 4, data, len);

  // queue function
  ESP_ERROR_CHECK(httpd_queue_work(naos_http_handle, naos_http_send_frame, msg));

  return true;
}

static httpd_uri_t naos_http_route = {
    .uri = "*",
    .method = HTTP_GET,
    .handler = naos_http_request,
    .is_websocket = true,
    .supported_subprotocol = "naos",
};

void naos_http_init(int core) {
  // prepare config
  httpd_config_t config = HTTPD_DEFAULT_CONFIG();
  config.max_open_sockets = NAOS_HTTP_MAX_CONNS;
  config.uri_match_fn = httpd_uri_match_wildcard;
  config.core_id = core;
  config.lru_purge_enable = true;

  // start server
  ESP_ERROR_CHECK(httpd_start(&naos_http_handle, &config));

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
