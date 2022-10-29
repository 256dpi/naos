#include <esp_http_server.h>

#include "params.h"
#include "utils.h"

#define NAOS_HTTP_MAX_CONNS 7

static httpd_handle_t naos_http_handle = {0};

extern const char naos_http_console[] asm("_binary_console_html_start");

static esp_err_t naos_http_index(httpd_req_t *req) {
  // set response header
  esp_err_t err = httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  if (err != ESP_OK) {
    return err;
  }

  // write response
  err = httpd_resp_send(req, naos_http_console, HTTPD_RESP_USE_STRLEN);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static esp_err_t naos_http_socket(httpd_req_t *conn) {
  // handle GET request
  if (conn->method == HTTP_GET) {
    return ESP_OK;
  }

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

  // handle list
  if (strncmp((char *)req.payload, "list", 4) == 0) {
    // list params
    char *list = naos_params_list(0);

    // set payload
    res.payload = (uint8_t *)naos_concat("list#", list);
    res.len = strlen((char *)res.payload);

    // free intermediaries
    free(list);
  }

  // handle read
  if (strncmp((char *)req.payload, "read", 4) == 0) {
    // get name
    const char *name = (char *)req.payload + 5;

    // lookup param
    naos_param_t *param = naos_lookup(name);
    if (param == NULL) {
      free(req.payload);
      return ESP_FAIL;
    }

    // get value
    char *value = strdup(naos_get(param->name));

    // set payload
    res.payload = (uint8_t *)naos_format("read:%s#%s", name, value);
    res.len = strlen((char *)res.payload);

    // free intermediates
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
    if (param == NULL) {
      free(req.payload);
      return ESP_FAIL;
    }

    // set value
    naos_set(param->name, value);

    // get value
    value = strdup(naos_get(param->name));

    // set payload
    res.payload = (uint8_t *)naos_format("write:%s#%s", name, value);
    res.len = strlen((char *)res.payload);

    // free intermediates
    free(value);
  }

  // free request payload
  free(req.payload);

  // send response frame
  err = httpd_ws_send_frame(conn, &res);

  // free response payload
  free(res.payload);

  return err;
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
static httpd_uri_t naos_http_route_socket = {.uri = "/naos.sock",
                                             .method = HTTP_GET,
                                             .handler = naos_http_socket,
                                             .is_websocket = true,
                                             .supported_subprotocol = "naos"};

void naos_http_init() {
  // prepare config
  httpd_config_t config = HTTPD_DEFAULT_CONFIG();
  config.max_open_sockets = NAOS_HTTP_MAX_CONNS;

  // start server
  ESP_ERROR_CHECK(httpd_start(&naos_http_handle, &config));

  // register handlers
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_index));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_socket));

  // handle parameters
  naos_params_subscribe(naos_http_param_handler);
}
