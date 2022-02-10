#include <esp_event.h>
#include <esp_event_loop.h>
#include <esp_wifi.h>
#include <string.h>

#include "utils.h"
#include "net.h"

static SemaphoreHandle_t naos_net_mutex;

static bool naos_wifi_started = false;
static bool naos_wifi_connected = false;
static bool naos_eth_connected = false;
static bool naos_net_connected = false;

static naos_net_status_callback_t naos_net_callback = NULL;

static wifi_config_t naos_wifi_config;

static esp_err_t naos_net_event_handler(void *ctx, system_event_t *e) {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  switch (e->event_id) {
    case SYSTEM_EVENT_STA_START: {
      // connect to access point if station has started
      ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());

      break;
    }

    case SYSTEM_EVENT_STA_GOT_IP: {
      // set flag
      naos_wifi_connected = true;

      break;
    }

    case SYSTEM_EVENT_STA_DISCONNECTED: {
      // set flag
      naos_wifi_connected = false;

      // attempt to reconnect if station is not down
      if (naos_wifi_started) {
        ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());
      }

      break;
    }

    case SYSTEM_EVENT_ETH_GOT_IP: {
      // set flag
      naos_eth_connected = true;

      break;
    }

    case SYSTEM_EVENT_ETH_DISCONNECTED: {
      // set flag
      naos_eth_connected = false;

      break;
    }

    default: {
      // ESP_LOGI(NAOS_LOG_TAG, "Unhandled Event: %d", e->event_id);
    }
  }

  // determine status
  bool connected = naos_wifi_connected || naos_eth_connected;
  bool changed = naos_net_connected != connected;

  // update status
  naos_net_connected = connected;

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);

  // call callback if changed and present
  if (naos_net_callback && changed) {
    naos_net_callback(connected ? NAOS_NET_STATUS_CONNECTED : NAOS_NET_STATUS_DISCONNECTED);
  }

  return ESP_OK;
}

void naos_net_init(naos_net_status_callback_t callback) {
  // store callback
  naos_net_callback = callback;

  // create mutex
  naos_net_mutex = xSemaphoreCreateMutex();

  // initialize TCP/IP adapter
  tcpip_adapter_init();

  // start event loop
  ESP_ERROR_CHECK(esp_event_loop_init(naos_net_event_handler, NULL));

  // get default Wi-Fi initialization config
  wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();

  // initialize Wi-Fi
  ESP_ERROR_CHECK(esp_wifi_init(&cfg));

  // use RAM storage for Wi-Fi config
  ESP_ERROR_CHECK(esp_wifi_set_storage(WIFI_STORAGE_RAM));

  // disable Wi-Fi auto connect
  esp_wifi_set_auto_connect(false);
}

void naos_net_configure_wifi(const char *ssid, const char *password) {
  // immediately return if ssid is not set
  if (strlen(ssid) == 0) {
    return;
  }

  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // stop station if already started
  if (naos_wifi_started) {
    ESP_ERROR_CHECK(esp_wifi_stop());
  }

  // assign ssid and password
  strcpy((char *)naos_wifi_config.sta.ssid, ssid);
  strcpy((char *)naos_wifi_config.sta.password, password);

  // set to station mode
  ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));

  // assign configuration
  ESP_ERROR_CHECK(esp_wifi_set_config(ESP_IF_WIFI_STA, &naos_wifi_config));

  // update flag
  naos_wifi_started = true;

  // start Wi-Fi
  ESP_ERROR_CHECK(esp_wifi_start());

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);
}
