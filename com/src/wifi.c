#include <naos.h>
#include <naos/sys.h>

#include <string.h>
#include <esp_event.h>
#include <esp_wifi.h>

#include "utils.h"
#include "net.h"

static naos_mutex_t naos_wifi_mutex;
static wifi_config_t naos_wifi_config;
static esp_netif_t *naos_wifi_netif;
static bool naos_wifi_started;
static bool naos_wifi_connected;
static uint16_t naos_wifi_generation = 0;
static char naos_wifi_addr[16] = {0};

static void naos_wifi_configure() {
  // log call
  ESP_LOGI(NAOS_LOG_TAG, "naos_wifi_configure");

  // acquire mutex
  NAOS_LOCK(naos_wifi_mutex);

  // stop station if already started
  if (naos_wifi_started) {
    ESP_ERROR_CHECK(esp_wifi_stop());
    naos_wifi_started = false;
  }

  // get SSID, password and manual config
  const char *ssid = naos_get_s("wifi-ssid");
  const char *password = naos_get_s("wifi-password");
  const char *manual = naos_get_s("wifi-manual");

  // return if SSID is missing
  if (strlen(ssid) == 0) {
    NAOS_UNLOCK(naos_wifi_mutex);
    return;
  }

  // configure network
  naos_net_configure(naos_wifi_netif, manual);

  // configure station
  strcpy((char *)naos_wifi_config.sta.ssid, ssid);
  strcpy((char *)naos_wifi_config.sta.password, password);
  ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));
  ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &naos_wifi_config));

  // start station
  ESP_ERROR_CHECK(esp_wifi_start());
  naos_wifi_started = true;

  // release mutex
  NAOS_UNLOCK(naos_wifi_mutex);
}

static void naos_wifi_handler(void *arg, esp_event_base_t base, int32_t id, void *data) {
  // acquire mutex
  NAOS_LOCK(naos_wifi_mutex);

  // handle WiFi events
  if (base == WIFI_EVENT) {
    switch (id) {
      case WIFI_EVENT_STA_START: {
        // initial connection
        ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());

        break;
      }

      case WIFI_EVENT_STA_DISCONNECTED: {
        // set status
        naos_wifi_connected = false;

        // clear addr
        memset(naos_wifi_addr, 0, 16);
        naos_set_s("wifi-addr", "");

        // attempt to reconnect if started
        if (naos_wifi_started) {
          ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());
        }

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled wifi event: %d", event_id);
      }
    }
  }

  // handle IP events
  if (base == IP_EVENT) {
    switch (id) {
      case IP_EVENT_STA_GOT_IP: {
        // set status
        naos_wifi_connected = true;
        naos_wifi_generation++;

        // set addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)data;
        naos_net_ip2str(&event->ip_info.ip, naos_wifi_addr);
        naos_set_s("wifi-addr", naos_wifi_addr);

        break;
      }

      default: {
        // ESP_LOGI(NAOS_LOG_TAG, "unhandled IP event: %d", event_id);
      }
    }
  }

  // release mutex
  NAOS_UNLOCK(naos_wifi_mutex);
}

static naos_net_status_t naos_wifi_status() {
  // read status
  NAOS_LOCK(naos_wifi_mutex);
  naos_net_status_t status = {
      .connected = naos_wifi_connected,
      .generation = naos_wifi_generation,
  };
  NAOS_UNLOCK(naos_wifi_mutex);

  return status;
}

static void naos_wifi_update() {
  // update RSSI
  wifi_ap_record_t record = {0};
  esp_wifi_sta_get_ap_info(&record);
  naos_set_l("wifi-rssi", record.rssi);
}

static naos_param_t naos_wifi_params[] = {
    {.name = "wifi-ssid", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "wifi-password", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "wifi-manual", .type = NAOS_STRING, .mode = NAOS_SYSTEM},
    {.name = "wifi-configure", .type = NAOS_ACTION, .mode = NAOS_SYSTEM, .func_a = naos_wifi_configure},
    {.name = "wifi-rssi", .type = NAOS_LONG, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "wifi-addr", .type = NAOS_STRING, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
};

void naos_wifi_init() {
  // create mutex
  naos_wifi_mutex = naos_mutex();

  // create WiFi station
  naos_wifi_netif = esp_netif_create_default_wifi_sta();

  // initialize WiFi
  wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
  ESP_ERROR_CHECK(esp_wifi_init(&cfg));

  // set WiFi storage to ram
  ESP_ERROR_CHECK(esp_wifi_set_storage(WIFI_STORAGE_RAM));

  // register event handlers
  ESP_ERROR_CHECK(esp_event_handler_instance_register(WIFI_EVENT, ESP_EVENT_ANY_ID, &naos_wifi_handler, NULL, NULL));
  ESP_ERROR_CHECK(esp_event_handler_instance_register(IP_EVENT, ESP_EVENT_ANY_ID, &naos_wifi_handler, NULL, NULL));

  // register link
  naos_net_link_t link = {
      .name = "wifi",
      .status = naos_wifi_status,
  };
  naos_net_register(link);

  // register parameters
  for (size_t i = 0; i < NAOS_NUM_PARAMS(naos_wifi_params); i++) {
    naos_register(&naos_wifi_params[i]);
  }

  // perform initial configuration
  naos_wifi_configure();

  // start update timer
  naos_repeat("naos-wifi", 1000, naos_wifi_update);
}
