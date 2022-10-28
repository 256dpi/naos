#include <esp_event.h>
#include <string.h>
#include <esp_wifi.h>
#include <freertos/FreeRTOS.h>
#include <freertos/timers.h>

#include <naos.h>

#include "utils.h"
#include "net.h"

static SemaphoreHandle_t naos_wifi_mutex;
static wifi_config_t naos_wifi_config;
static esp_netif_t *naos_wifi_netif;
static bool naos_wifi_started;
static bool naos_wifi_connected;
static char naos_wifi_addr[16] = {0};
static TimerHandle_t naos_wifi_timer;

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
  const char *ssid = naos_get("wifi-ssid");
  const char *password = naos_get("wifi-password");
  const char *manual = naos_get("wifi-manual");

  // return if SSID is missing
  if (strlen(ssid) == 0) {
    NAOS_UNLOCK(naos_wifi_mutex);
    return;
  }

  // configure network
  naos_net_configure(naos_wifi_netif, manual);

  // assign SSID and password
  strcpy((char *)naos_wifi_config.sta.ssid, ssid);
  strcpy((char *)naos_wifi_config.sta.password, password);

  // set to station mode
  ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));

  // assign configuration
  ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &naos_wifi_config));

  // update flag
  naos_wifi_started = true;

  // start WiFi
  ESP_ERROR_CHECK(esp_wifi_start());

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
        naos_set("wifi-addr", "");

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

        // set addr
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)data;
        naos_net_ip2str(&event->ip_info.ip, naos_wifi_addr);
        naos_set("wifi-addr", naos_wifi_addr);

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
  naos_net_status_t status = {.connected = naos_wifi_connected};
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
  naos_wifi_mutex = xSemaphoreCreateMutex();

  // enable WiFi
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
  naos_net_link_t link = {.status = naos_wifi_status};
  naos_net_register(link);

  // register parameters
  for (size_t i = 0; i < (sizeof(naos_wifi_params) / sizeof(naos_wifi_params[0])); i++) {
    naos_register(&naos_wifi_params[i]);
  }

  // perform initial configuration
  naos_wifi_configure();

  // start update timer
  naos_wifi_timer = xTimerCreate("naos-wifi", pdMS_TO_TICKS(1000), pdTRUE, 0, naos_wifi_update);
  xTimerStart(naos_wifi_timer, 0);
}
