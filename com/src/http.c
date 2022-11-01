#include <esp_http_server.h>

#include "params.h"
#include "utils.h"
#include "naos.h"

#define NAOS_HTTP_MAX_CONNS 7
#define NAOS_HTTP_MAX_FILES 8

typedef struct {
  bool locked;
} naos_http_ctx_t;

typedef struct {
  const char *path;
  const char *type;
  const char *content;
} naos_http_file_t;

extern const char naos_http_index_html[] asm("_binary_naos_html_start");
extern const char naos_http_script_js[] asm("_binary_naos_js_start");

static httpd_handle_t naos_http_handle = {0};
static naos_http_file_t naos_http_files[NAOS_HTTP_MAX_FILES] = {0};
static size_t naos_http_file_count = 0;

static esp_err_t naos_http_index(httpd_req_t *req) {
  // set response header
  esp_err_t err = httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  if (err != ESP_OK) {
    return err;
  }

  // write response
  err = httpd_resp_sendstr(req, naos_http_index_html);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static esp_err_t naos_http_script(httpd_req_t *req) {
  // set response header
  esp_err_t err = httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  if (err != ESP_OK) {
    return err;
  }

  // set content type
  err = httpd_resp_set_type(req, "text/javascript");
  if (err != ESP_OK) {
    return err;
  }

  // write response
  err = httpd_resp_sendstr(req, naos_http_script_js);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static esp_err_t naos_http_socket(httpd_req_t *conn) {
  // handle GET request
  if (conn->method == HTTP_GET) {
    // set context
    naos_http_ctx_t *ctx = malloc(sizeof(naos_http_ctx_t));
    ctx->locked = strlen(naos_get("device-password")) > 0;
    conn->sess_ctx = ctx;

    return ESP_OK;
  }

  // get context
  naos_http_ctx_t *ctx = conn->sess_ctx;

  // prepare request frame
  httpd_ws_frame_t req = {.type = HTTPD_WS_TYPE_TEXT};

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

  // prepare response
  httpd_ws_frame_t res = {.type = HTTPD_WS_TYPE_TEXT};

  // handle ping
  if (strcmp((char *)req.payload, "ping") == 0) {
    res.payload = (uint8_t *)strdup("ping");
  }

  // handle lock
  if (strcmp((char *)req.payload, "lock") == 0) {
    res.payload = (uint8_t *)strdup(ctx->locked ? "lock#locked" : "lock#unlocked");
  }

  // handle unlock
  if (strncmp((char *)req.payload, "unlock", 6) == 0) {
    const char *password = (char *)req.payload + 7;
    naos_log("unlock with '%s'", password);
    if (ctx->locked) {
      ctx->locked = strcmp(password, naos_get("device-password")) != 0;
    }
    res.payload = (uint8_t *)strdup(ctx->locked ? "unlock#locked" : "unlock#unlocked");
  }

  // handle list
  if (strcmp((char *)req.payload, "list") == 0) {
    char *list = naos_params_list(ctx->locked ? NAOS_PUBLIC : 0);
    res.payload = (uint8_t *)naos_concat("list#", list);
    free(list);
  }

  // handle read
  if (strncmp((char *)req.payload, "read", 4) == 0) {
    // get name
    const char *name = (char *)req.payload + 5;

    // lookup param
    naos_param_t *param = naos_lookup(name);
    if (param == NULL || (ctx->locked && !(param->mode & NAOS_PUBLIC))) {
      free(req.payload);
      return ESP_FAIL;
    }

    // set value
    char *value = strdup(naos_get(param->name));
    res.payload = (uint8_t *)naos_format("read:%s#%s", name, value);
    free(value);
  }

  // handle write
  if (strncmp((char *)req.payload, "write", 5) == 0 && strchr((char *)req.payload, '#') != NULL) {
    // get name
    const char *name = (char *)req.payload + 6;

    // replace hash with zero
    char *hash = strchr(name, '#');
    *hash = 0;

    // get value
    char *value = hash + 1;

    // lookup param
    naos_param_t *param = naos_lookup(name);
    if (param == NULL || (ctx->locked && !(param->mode & NAOS_PUBLIC))) {
      free(req.payload);
      return ESP_FAIL;
    }

    // set value
    naos_set(param->name, value);

    // set value
    value = strdup(naos_get(param->name));
    res.payload = (uint8_t *)naos_format("write:%s#%s", name, value);
    free(value);
  }

  // free request payload
  free(req.payload);

  // send response frame
  res.len = strlen((char *)res.payload);
  err = httpd_ws_send_frame(conn, &res);

  // free response payload
  free(res.payload);

  return err;
}

static esp_err_t naos_http_file(httpd_req_t *req) {
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

    // send response
    err = httpd_resp_sendstr(req, file->content);
    if (err != ESP_OK) {
      return err;
    }

    return ESP_OK;
  }

  // redirect if root is missing
  if (strcmp(req->uri, "/") == 0) {
    err = httpd_resp_set_status(req, "302 Found");
    if (err != ESP_OK) {
      return err;
    }
    err = httpd_resp_set_hdr(req, "Location", "/naos");
    if (err != ESP_OK) {
      return err;
    }
    err = httpd_resp_send(req, NULL, 0);
    if (err != ESP_OK) {
      return err;
    }
  }

  // serve root if available
  err = httpd_resp_send_404(req);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static void naos_http_update(void *arg) {
  // get param
  naos_param_t *param = arg;

  // get sessions
  size_t num = NAOS_HTTP_MAX_CONNS;
  int fds[NAOS_HTTP_MAX_CONNS] = {0};
  ESP_ERROR_CHECK(httpd_get_client_list(naos_http_handle, &num, fds));

  // prepare frame
  httpd_ws_frame_t frame = {.type = HTTPD_WS_TYPE_TEXT};
  frame.payload = (uint8_t *)naos_format("update:%s#%s", param->name, param->value);
  frame.len = strlen((char *)frame.payload);

  // iterate sessions
  for (size_t i = 0; i < num; i++) {
    // check if websocket
    if (httpd_ws_get_fd_info(naos_http_handle, fds[i]) != HTTPD_WS_CLIENT_WEBSOCKET) {
      continue;
    }

    // check context
    naos_http_ctx_t *ctx = httpd_sess_get_ctx(naos_http_handle, fds[i]);
    if (ctx == NULL || ctx->locked) {
      continue;
    }

    // send frame
    ESP_ERROR_CHECK_WITHOUT_ABORT(httpd_ws_send_frame_async(naos_http_handle, fds[i], &frame));
  }

  // free payload
  free(frame.payload);
}

static void naos_http_param_handler(naos_param_t *param) {
  // queue function
  ESP_ERROR_CHECK(httpd_queue_work(naos_http_handle, naos_http_update, param));
}

static httpd_uri_t naos_http_route_index = {.uri = "/naos", .method = HTTP_GET, .handler = naos_http_index};
static httpd_uri_t naos_http_route_script = {.uri = "/naos.js", .method = HTTP_GET, .handler = naos_http_script};
static httpd_uri_t naos_http_route_socket = {.uri = "/naos.sock",
                                             .method = HTTP_GET,
                                             .handler = naos_http_socket,
                                             .is_websocket = true,
                                             .supported_subprotocol = "naos"};
static httpd_uri_t naos_http_route_file = {.uri = "*", .method = HTTP_GET, .handler = naos_http_file};

void naos_http_init() {
  // prepare config
  httpd_config_t config = HTTPD_DEFAULT_CONFIG();
  config.max_open_sockets = NAOS_HTTP_MAX_CONNS;
  config.uri_match_fn = httpd_uri_match_wildcard;

  // start server
  ESP_ERROR_CHECK(httpd_start(&naos_http_handle, &config));

  // register handlers
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_index));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_script));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_socket));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_file));

  // handle parameters
  naos_params_subscribe(naos_http_param_handler);
}

void naos_http_serve(const char *path, const char *type, const char *content) {
  // check count
  if (naos_http_file_count >= NAOS_HTTP_MAX_FILES) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // prepare files
  naos_http_file_t file = {
      .path = path,
      .type = type,
      .content = content,
  };

  // store file
  naos_http_files[naos_http_file_count] = file;
  naos_http_file_count++;
}
