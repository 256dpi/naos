#include <esp_event.h>
#include <esp_wifi.h>
#include <string.h>

#include "utils.h"
#include "net.h"

static SemaphoreHandle_t naos_net_mutex;

static naos_net_status_t naos_net_status = {0};

static wifi_config_t naos_wifi_config;

static void naos_net_event_handler(void *event_handler_arg, esp_event_base_t event_base, int32_t event_id,
                                        void *event_data) {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // handle Wi-Fi events
  if (event_base == WIFI_EVENT) {
    switch (event_id) {
      case WIFI_EVENT_STA_START: {
        // initial connection
        ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());

        break;
      }

      case WIFI_EVENT_STA_DISCONNECTED: {
        // set status
        naos_net_status.connected_wifi = false;

        // attempt to reconnect
        ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "Unhandled Event: %d", event_id);
      }
    }
  }

  // handle ethernet events
  if (event_base == ETH_EVENT) {
    switch (event_id) {
      case ETHERNET_EVENT_DISCONNECTED: {
        // set status
        naos_net_status.connected_eth = false;

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "Unhandled Event: %d", event_id);
      }
    }
  }

  // handle IP events
  if (event_base == IP_EVENT) {
    switch(event_id) {
      case IP_EVENT_STA_GOT_IP: {
        // set status
        naos_net_status.connected_wifi = true;

        break;
      }

      case IP_EVENT_ETH_GOT_IP: {
        // set status
        naos_net_status.connected_eth = true;

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "Unhandled Event: %d", event_id);
      }
    }
  }

  // set any status
  naos_net_status.connected_any = naos_net_status.connected_wifi || naos_net_status.connected_eth;

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);
}

void naos_net_init() {
  // create mutex
  naos_net_mutex = xSemaphoreCreateMutex();

  // initialize networking
  ESP_ERROR_CHECK(esp_netif_init());

  // create default event loop
  ESP_ERROR_CHECK(esp_event_loop_create_default());

  // enable Wi-Fi
  esp_netif_create_default_wifi_sta();

  // initialize Wi-Fi
  wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
  ESP_ERROR_CHECK(esp_wifi_init(&cfg));

  // set Wi-Fi storage to ram
  ESP_ERROR_CHECK(esp_wifi_set_storage(WIFI_STORAGE_RAM));

  // register event handlers
  ESP_ERROR_CHECK(esp_event_handler_instance_register(WIFI_EVENT, ESP_EVENT_ANY_ID, &naos_net_event_handler, NULL, NULL));
  ESP_ERROR_CHECK(esp_event_handler_instance_register(ETH_EVENT, ESP_EVENT_ANY_ID, &naos_net_event_handler, NULL, NULL));
  ESP_ERROR_CHECK(esp_event_handler_instance_register(IP_EVENT, IP_EVENT_STA_GOT_IP, &naos_net_event_handler, NULL, NULL));
}

void naos_net_configure_wifi(const char *ssid, const char *password) {
  static bool started = false;

  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // stop station if already started
  if (started) {
    ESP_ERROR_CHECK(esp_wifi_stop());
    started = false;
  }

  // return if ssid is not set
  if (strlen(ssid) == 0) {
    NAOS_UNLOCK(naos_net_mutex);
    return;
  }

  // assign ssid and password
  strcpy((char *)naos_wifi_config.sta.ssid, ssid);
  strcpy((char *)naos_wifi_config.sta.password, password);

  // set to station mode
  ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));

  // assign configuration
  ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &naos_wifi_config));

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
