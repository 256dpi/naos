#include <esp_event.h>
#include <string.h>

#ifndef CONFIG_NAOS_WIFI_DISABLE
#include <esp_wifi.h>
#endif

#include "utils.h"
#include "net.h"

static SemaphoreHandle_t naos_net_mutex;

static naos_net_status_t naos_net_status = {0};

#ifndef CONFIG_NAOS_WIFI_DISABLE
static wifi_config_t naos_wifi_config;
#endif

static void naos_net_event_handler(void *event_handler_arg, esp_event_base_t event_base, int32_t event_id,
                                   void *event_data) {
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // handle Wi-Fi events
#ifndef CONFIG_NAOS_WIFI_DISABLE
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

        // clear ip
        memset(naos_net_status.ip_wifi, 0, 16);

        // attempt to reconnect if started
        if (naos_net_status.started_wifi) {
          ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());
        }

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled wifi event: %d", event_id);
      }
    }
  }
#endif

  // handle ethernet events
  if (event_base == ETH_EVENT) {
    switch (event_id) {
      case ETHERNET_EVENT_DISCONNECTED: {
        // set status
        naos_net_status.connected_eth = false;

        // clear ip
        memset(naos_net_status.ip_eth, 0, 16);

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled ethernet event: %d", event_id);
      }
    }
  }

  // handle IP events
  if (event_base == IP_EVENT) {
    switch (event_id) {
      case IP_EVENT_STA_GOT_IP: {
        // set status
        naos_net_status.connected_wifi = true;

        // set ip addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)event_data;
        sprintf(naos_net_status.ip_wifi, IPSTR, IP2STR(&event->ip_info.ip));

        break;
      }

      case IP_EVENT_ETH_GOT_IP: {
        // set status
        naos_net_status.connected_eth = true;

        // set ip addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)event_data;
        sprintf(naos_net_status.ip_eth, IPSTR, IP2STR(&event->ip_info.ip));

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled IP event: %d", event_id);
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

#ifndef CONFIG_NAOS_WIFI_DISABLE
  // enable Wi-Fi
  esp_netif_create_default_wifi_sta();

  // initialize Wi-Fi
  wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
  ESP_ERROR_CHECK(esp_wifi_init(&cfg));

  // set Wi-Fi storage to ram
  ESP_ERROR_CHECK(esp_wifi_set_storage(WIFI_STORAGE_RAM));
#endif

  // register event handlers
  ESP_ERROR_CHECK(
      esp_event_handler_instance_register(WIFI_EVENT, ESP_EVENT_ANY_ID, &naos_net_event_handler, NULL, NULL));
  ESP_ERROR_CHECK(
      esp_event_handler_instance_register(ETH_EVENT, ESP_EVENT_ANY_ID, &naos_net_event_handler, NULL, NULL));
  ESP_ERROR_CHECK(esp_event_handler_instance_register(IP_EVENT, ESP_EVENT_ANY_ID, &naos_net_event_handler, NULL, NULL));
}

void naos_net_configure_wifi(const char *ssid, const char *password) {
#ifndef CONFIG_NAOS_WIFI_DISABLE
  // acquire mutex
  NAOS_LOCK(naos_net_mutex);

  // stop station if already started
  if (naos_net_status.started_wifi) {
    ESP_ERROR_CHECK(esp_wifi_stop());
    naos_net_status.started_wifi = false;
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
  naos_net_status.started_wifi = true;

  // start Wi-Fi
  ESP_ERROR_CHECK(esp_wifi_start());

  // release mutex
  NAOS_UNLOCK(naos_net_mutex);
#endif
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
