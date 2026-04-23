#include <naos.h>
#include <naos/sys.h>
#include <naos/msg.h>
#include <naos/time.h>

#include <time.h>
#include <sys/time.h>
#include <esp_err.h>
#include <esp_log.h>
#include <esp_netif_sntp.h>
#include <esp_sntp.h>

#include "system.h"
#include "tzdata.h"
#include "utils.h"

#define NAOS_TIME_ENDPOINT 0x9
#define NAOS_TIME_SNTP_CHECK_MS 30000
#define NAOS_TIME_SNTP_GRACE_MS 60000

typedef enum {
  NAOS_TIME_CMD_GET = 0x00,
  NAOS_TIME_CMD_SET = 0x01,
  NAOS_TIME_CMD_INFO = 0x02,
} naos_time_cmd_t;

static naos_mutex_t naos_time_mutex;
static bool naos_time_sntp_running = false;
static int64_t naos_time_sntp_start_ms = 0;
static char *naos_time_sntp_buf = NULL;

static void naos_time_sntp_cb(struct timeval *tv) {
  // set status
  naos_set_s("time-sntp-status", "synced");
}

static int naos_time_tzdata_compare(const void *key, const void *entry) {
  return strcmp((const char *)key, ((const naos_tzdata_entry_t *)entry)->name);
}

static void naos_time_apply_tz_name(const char *name) {
  // look up POSIX string for the given IANA name
  const char *posix = NULL;
  if (name != NULL && *name != 0) {
    const naos_tzdata_entry_t *entry =
        bsearch(name, naos_tzdata, naos_tzdata_count, sizeof(*naos_tzdata), naos_time_tzdata_compare);
    if (entry != NULL) {
      posix = entry->posix;
    } else {
      ESP_LOGW(NAOS_LOG_TAG, "naos_time_apply_tz_name: unknown timezone '%s', falling back to UTC0", name);
    }
  }

  // publish derived POSIX string (empty if lookup failed or name was empty)
  naos_set_s("time-tz-posix", posix != NULL ? posix : "");

  // update timezone (deterministic UTC0 on fallback)
  setenv("TZ", posix != NULL ? posix : "UTC0", 1);
  tzset();
}

static void naos_time_sntp_start_locked(const char *list) {
  // duplicate list for destructive tokenization
  naos_time_sntp_buf = strdup(list);
  if (naos_time_sntp_buf == NULL) {
    ESP_ERROR_CHECK(ESP_ERR_NO_MEM);
  }

  // parse up to SNTP_MAX_SERVERS entries
  const char *servers[CONFIG_LWIP_SNTP_MAX_SERVERS] = {0};
  size_t num = 0;
  char *save = NULL;
  char *tok = strtok_r(naos_time_sntp_buf, ",", &save);
  while (tok != NULL && num < CONFIG_LWIP_SNTP_MAX_SERVERS) {
    // skip leading whitespace
    while (*tok == ' ' || *tok == '\t') tok++;
    if (*tok != 0) {
      servers[num++] = tok;
    }
    tok = strtok_r(NULL, ",", &save);
  }
  if (tok != NULL) {
    ESP_LOGW(NAOS_LOG_TAG, "naos_time_sntp_start: server list exceeds SNTP_MAX_SERVERS=%d, truncated",
             CONFIG_LWIP_SNTP_MAX_SERVERS);
  }

  // stop if no usable servers parsed
  if (num == 0) {
    free(naos_time_sntp_buf);
    naos_time_sntp_buf = NULL;
    return;
  }

  // prepare SNTP config
  esp_sntp_config_t config = {
      .smooth_sync = false,
      .server_from_dhcp = false,
      .wait_for_sync = false,
      .start = true,
      .sync_cb = naos_time_sntp_cb,
      .renew_servers_after_new_IP = false,
      .num_of_servers = num,
  };
  for (size_t i = 0; i < num; i++) {
    config.servers[i] = servers[i];
  }

  // init SNTP client
  ESP_ERROR_CHECK(esp_netif_sntp_init(&config));
  naos_time_sntp_running = true;
  naos_time_sntp_start_ms = naos_millis();
}

static void naos_time_sntp_stop_locked(void) {
  // stop client
  if (naos_time_sntp_running) {
    esp_netif_sntp_deinit();
    naos_time_sntp_running = false;
  }

  // free buffer
  if (naos_time_sntp_buf != NULL) {
    free(naos_time_sntp_buf);
    naos_time_sntp_buf = NULL;
  }
}

static void naos_time_sntp_sync(naos_status_t status, bool force) {
  // read configured list
  const char *list = naos_get_s("time-sntp-list");
  bool has_list = (list != NULL && list[0] != 0);
  bool want_run = has_list && (status >= NAOS_CONNECTED);

  // acquire mutex
  naos_lock(naos_time_mutex);

  // apply state change
  bool started = false;
  if (want_run && (!naos_time_sntp_running || force)) {
    if (naos_time_sntp_running) {
      naos_time_sntp_stop_locked();
    }
    naos_time_sntp_start_locked(list);
    started = naos_time_sntp_running;
  } else if (!want_run && naos_time_sntp_running) {
    naos_time_sntp_stop_locked();
  }
  bool running = naos_time_sntp_running;

  // release mutex
  naos_unlock(naos_time_mutex);

  // update status
  const char *desired = NULL;
  if (started) {
    desired = "unsynced";
  } else if (!running) {
    desired = has_list ? "offline" : "disabled";
  }
  if (desired != NULL) {
    naos_set_s("time-sntp-status", desired);
  }
}

static void naos_time_apply_sntp(const char *list) {
  // force SNTP client restart
  naos_time_sntp_sync(naos_status(), true);
}

static void naos_time_manage(naos_status_t status) {
  // sync SNTP client
  naos_time_sntp_sync(status, false);
}

static void naos_time_sntp_check(void) {
  // check reachability
  naos_lock(naos_time_mutex);
  bool running = naos_time_sntp_running;
  int64_t started = naos_time_sntp_start_ms;
  unsigned int reach = 0;
  esp_err_t err = ESP_OK;
  if (running && naos_millis() - started >= NAOS_TIME_SNTP_GRACE_MS) {
    err = esp_netif_sntp_reachability(0, &reach);
  } else {
    running = false;
  }
  naos_unlock(naos_time_mutex);

  // bail if not running, still in grace, or probe failed
  if (!running || err != ESP_OK) {
    return;
  }

  // update status (bit 0 = most recent attempt)
  const char *desired = (reach & 1) ? "synced" : "failed";
  const char *current = naos_get_s("time-sntp-status");
  if (current == NULL || strcmp(current, desired) != 0) {
    naos_set_s("time-sntp-status", desired);
  }
}

static naos_msg_reply_t naos_time_handle_get(naos_msg_t msg) {
  // reply structure:
  // MS (8, LE int64)

  // check length
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // get time as epoch MS
  struct timeval tv;
  gettimeofday(&tv, NULL);
  int64_t ms = (int64_t)tv.tv_sec * 1000 + tv.tv_usec / 1000;

  // write epoch MS to buffer
  uint8_t buf[8];
  for (int i = 0; i < 8; i++) {
    buf[i] = (uint8_t)((ms >> (i * 8)) & 0xff);
  }

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_TIME_ENDPOINT,
      .data = buf,
      .len = sizeof(buf),
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_time_handle_set(naos_msg_t msg) {
  // command structure:
  // MS (8, LE int64)

  // check length
  if (msg.len != 8) {
    return NAOS_MSG_INVALID;
  }

  // read epoch MS from buffer
  int64_t ms = 0;
  for (int i = 0; i < 8; i++) {
    ms |= (int64_t)msg.data[i] << (i * 8);
  }

  // set time from epoch MS
  struct timeval tv = {
      .tv_sec = (time_t)(ms / 1000),
      .tv_usec = (suseconds_t)((ms % 1000) * 1000),
  };
  if (settimeofday(&tv, NULL) != 0) {
    return NAOS_MSG_ERROR;
  }

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_time_handle_info(naos_msg_t msg) {
  // reply structure:
  // OFFSET (4, LE int32)

  // check length
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // compute TZ offset (seconds east of UTC, DST-aware via tm_isdst=-1, tm_gmtoff unavailable)
  time_t now;
  time(&now);
  struct tm gm;
  gmtime_r(&now, &gm);
  gm.tm_isdst = -1;
  time_t local_as_utc = mktime(&gm);
  if (local_as_utc == (time_t)-1) {
    return NAOS_MSG_ERROR;
  }
  int32_t offset = (int32_t)(now - local_as_utc);

  // write offset to buffer
  uint8_t buf[4];
  for (int i = 0; i < 4; i++) {
    buf[i] = (uint8_t)((offset >> (i * 8)) & 0xff);
  }

  // send reply
  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_TIME_ENDPOINT,
      .data = buf,
      .len = sizeof(buf),
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_time_process(naos_msg_t msg) {
  // message structure:
  // CMD (1) | *

  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // get command
  naos_time_cmd_t cmd = msg.data[0];

  // resize message
  msg.data++;
  msg.len--;

  // handle command
  switch (cmd) {
    case NAOS_TIME_CMD_GET:
      return naos_time_handle_get(msg);
    case NAOS_TIME_CMD_SET:
      return naos_time_handle_set(msg);
    case NAOS_TIME_CMD_INFO:
      return naos_time_handle_info(msg);
    default:
      return NAOS_MSG_UNKNOWN;
  }
}

static naos_param_t naos_time_params[] = {
    {.name = "time-tz-name",
     .type = NAOS_STRING,
     .mode = NAOS_SYSTEM,
     .default_s = "Etc/UTC",
     .func_s = naos_time_apply_tz_name,
     .skip_func_init = true},
    {.name = "time-tz-posix",
     .type = NAOS_STRING,
     .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED,
     .default_s = "UTC0"},
    {.name = "time-sntp-list",
     .type = NAOS_STRING,
     .mode = NAOS_SYSTEM,
     .default_s = "pool.ntp.org",
     .func_s = naos_time_apply_sntp,
     .skip_func_init = true},
    {.name = "time-sntp-status",
     .type = NAOS_STRING,
     .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED,
     .default_s = "disabled"},
};

void naos_time_init(void) {
  // create mutex
  naos_time_mutex = naos_mutex();

  // register parameters
  for (size_t i = 0; i < NAOS_COUNT(naos_time_params); i++) {
    naos_register(&naos_time_params[i]);
  }

  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_TIME_ENDPOINT,
      .name = "time",
      .handle = naos_time_process,
  });

  // apply current TZ
  naos_time_apply_tz_name(naos_get_s("time-tz-name"));

  // set initial SNTP status
  const char *list = naos_get_s("time-sntp-list");
  naos_set_s("time-sntp-status", (list != NULL && list[0] != 0) ? "offline" : "disabled");

  // gate SNTP client on connectivity
  naos_system_subscribe(naos_time_manage);

  // periodic reachability probe
  naos_repeat_defer("naos-time-sntp", NAOS_TIME_SNTP_CHECK_MS, naos_time_sntp_check);
}
