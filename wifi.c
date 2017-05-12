#include <esp_event.h>
#include <esp_event_loop.h>
#include <esp_wifi.h>
#include <string.h>

#include "general.h"
#include "wifi.h"

static SemaphoreHandle_t nadk_wifi_mutex;

static bool nadk_wifi_connected = false;
static bool nadk_wifi_started = false;

static nadk_wifi_callback_t nadk_wifi_callback = NULL;

static wifi_config_t nadk_wifi_config;

static esp_err_t nadk_wifi_event_handler(void *ctx, system_event_t *e) {
  // acquire mutex
  NADK_LOCK(nadk_wifi_mutex);

  switch (e->event_id) {
    case SYSTEM_EVENT_STA_START: {
      // connect to access point if station has started
      ESP_ERROR_CHECK(esp_wifi_connect());

      break;
    }

    case SYSTEM_EVENT_STA_GOT_IP: {
      // update local flag if changed
      if (!nadk_wifi_connected) {
        nadk_wifi_connected = true;

        // release mutex
        NADK_UNLOCK(nadk_wifi_mutex);

        // call callback if available
        if (nadk_wifi_callback) {
          nadk_wifi_callback(NADK_WIFI_STATUS_CONNECTED);
        }

        return ESP_OK;
      }

      break;
    }

    case SYSTEM_EVENT_STA_DISCONNECTED: {
      // attempt reconnect if station is not down
      if (nadk_wifi_started) {
        ESP_ERROR_CHECK(esp_wifi_connect());
      }

      // update local flag if changed
      if (nadk_wifi_connected) {
        nadk_wifi_connected = false;

        // release mutex
        NADK_UNLOCK(nadk_wifi_mutex);

        // call callback if available
        if (nadk_wifi_callback) {
          nadk_wifi_callback(NADK_WIFI_STATUS_DISCONNECTED);
        }

        return ESP_OK;
      }

      break;
    }

    default: { break; }
  }

  // release mutex
  NADK_UNLOCK(nadk_wifi_mutex);

  return ESP_OK;
}

void nadk_wifi_init(nadk_wifi_callback_t callback) {
  // store callback
  nadk_wifi_callback = callback;

  // create mutex
  nadk_wifi_mutex = xSemaphoreCreateMutex();

  // init tcpip adapter
  tcpip_adapter_init();

  // start event loop
  ESP_ERROR_CHECK(esp_event_loop_init(nadk_wifi_event_handler, NULL));

  // get default wifi initialization config
  wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();

  // initialize wifi
  ESP_ERROR_CHECK(esp_wifi_init(&cfg));

  // use RAM storage
  ESP_ERROR_CHECK(esp_wifi_set_storage(WIFI_STORAGE_RAM));

  // disable auto connect
  esp_wifi_set_auto_connect(false);
}

void nadk_wifi_configure(const char *ssid, const char *password) {
  // immediately return if ssid is not set
  if (strlen(ssid) == 0) {
    return;
  }

  // acquire mutex
  NADK_LOCK(nadk_wifi_mutex);

  // stop station if already started
  if (nadk_wifi_started) {
    // update local flag
    nadk_wifi_started = false;

    // stop wifi
    ESP_ERROR_CHECK(esp_wifi_stop());
  }

  // assign ssid and password
  strcpy((char *)nadk_wifi_config.sta.ssid, ssid);
  strcpy((char *)nadk_wifi_config.sta.password, password);

  // set to station mode
  ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));

  // assign configuration
  ESP_ERROR_CHECK(esp_wifi_set_config(ESP_IF_WIFI_STA, &nadk_wifi_config));

  // update local flag
  nadk_wifi_started = true;

  // start wifi
  ESP_ERROR_CHECK(esp_wifi_start());

  // release mutex
  NADK_UNLOCK(nadk_wifi_mutex);
}
