#include <esp_http_server.h>

#include "params.h"

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

static esp_err_t naos_http_param(httpd_req_t *req) {
  // set response header
  esp_err_t err = httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  if (err != ESP_OK) {
    return err;
  }
  err = httpd_resp_set_hdr(req, "Access-Control-Allow-Methods", "*");
  if (err != ESP_OK) {
    return err;
  }

  // handle preflight
  if (req->method == HTTP_OPTIONS) {
    return httpd_resp_send(req, NULL, 0);
  }

  // read name
  char name[32] = {0};
  if (httpd_req_get_url_query_len(req) > 0) {
    char query[64] = {0};
    if (httpd_req_get_url_query_len(req) > sizeof(query)) {
      return httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "query too long");
    }
    err = httpd_req_get_url_query_str(req, query, sizeof(query));
    if (err != ESP_OK) {
      return err;
    }
    err = httpd_query_key_value(query, "name", name, sizeof(name));
    if (err != ESP_OK) {
      return err;
    }
  }

  // handle empty
  if (strnlen(name, sizeof(name)) <= 0) {
    // write response
    char *list = naos_params_list(0);
    err = httpd_resp_send(req, list, HTTPD_RESP_USE_STRLEN);
    free(list);
    if (err != ESP_OK) {
      return err;
    }

    return ESP_OK;
  }

  // find param
  naos_param_t *param = naos_lookup(name);
  if (param == NULL) {
    return httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "unknown param");
  }

  // handle read
  if (req->method == HTTP_GET) {
    // read param
    char *value = strdup(naos_get(name));

    // write value
    err = httpd_resp_sendstr(req, value);

    // free value
    free(value);

    return err;
  }

  /* handle write */

  // read value
  char value[128] = {0};
  if (req->content_len > sizeof(value)) {
    return httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "body too long");
  }
  size_t len = httpd_req_recv(req, value, sizeof(value));
  if (len != req->content_len) {
    return httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "body too short");
  }
  value[len] = 0;

  // write param
  naos_set(name, value);

  // write response
  err = httpd_resp_send(req, "OK", HTTPD_RESP_USE_STRLEN);
  if (err != ESP_OK) {
    return err;
  }

  return ESP_OK;
}

static httpd_uri_t naos_http_route_index = {.uri = "/", .method = HTTP_GET, .handler = naos_http_index};
static httpd_uri_t naos_http_route_param_opt = {.uri = "/naos", .method = HTTP_OPTIONS, .handler = naos_http_param};
static httpd_uri_t naos_http_route_param_get = {.uri = "/naos", .method = HTTP_GET, .handler = naos_http_param};
static httpd_uri_t naos_http_route_param_put = {.uri = "/naos", .method = HTTP_PUT, .handler = naos_http_param};

void naos_http_init() {
  // prepare config
  httpd_config_t config = HTTPD_DEFAULT_CONFIG();

  // start server
  ESP_ERROR_CHECK(httpd_start(&naos_http_handle, &config));

  // register handlers
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_index));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_param_opt));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_param_get));
  ESP_ERROR_CHECK(httpd_register_uri_handler(naos_http_handle, &naos_http_route_param_put));
}
