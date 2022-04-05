#include <esp_event.h>
#include <esp_event_loop.h>
#include <esp_wifi.h>
#include <string.h>

#include "utils.h"
#include "net.h"

static SemaphoreHandle_t naos_net_mutex;

static naos_net_status_t naos_net_status = {0};

static wifi_config_t naos_wifi_config;

static esp_err_t naos_net_event_handler(void *ctx, system_event_t *e) {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  switch (e->event_id) {
    case SYSTEM_EVENT_STA_START: {
      // initial connection
      ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());

      break;
    }

    case SYSTEM_EVENT_STA_GOT_IP: {
      // set status
      naos_net_status.connected_wifi = true;

      break;
    }

    case SYSTEM_EVENT_STA_DISCONNECTED: {
      // set status
      naos_net_status.connected_wifi = false;

      // attempt to reconnect
      ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());

      break;
    }

    case SYSTEM_EVENT_ETH_GOT_IP: {
      // set status
      naos_net_status.connected_eth = true;

      break;
    }

    case SYSTEM_EVENT_ETH_DISCONNECTED: {
      // set status
      naos_net_status.connected_eth = false;

      break;
    }

    default: {
      // ESP_LOGI(NAOS_LOG_TAG, "Unhandled Event: %d", e->event_id);
    }
  }

  // set any status
  naos_net_status.connected_any = naos_net_status.connected_wifi || naos_net_status.connected_eth;

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);

  return ESP_OK;
}

void naos_net_init() {
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
}

void naos_net_configure_wifi(const char *ssid, const char *password) {
  static bool started = false;

  // immediately return if ssid is not set
  if (strlen(ssid) == 0) {
    return;
  }

  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // stop station if already started
  if (started) {
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
  started = true;

  // start Wi-Fi
  ESP_ERROR_CHECK(esp_wifi_start());

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);
}

naos_net_status_t naos_net_check() {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // get status
  naos_net_status_t status = naos_net_status;

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);

  return status;
}
