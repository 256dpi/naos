#include <stdlib.h>
#include <string.h>

#include <esp_timer.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>

#include <naos/msg.h>
#include <naos/sys.h>
#include <naos/trace.h>

#define NAOS_TRACE_ENDPOINT 0x8
#define NAOS_TRACE_MAX_TASKS 64
#define NAOS_TRACE_MAX_LABELS 255
#define NAOS_TRACE_BUF_SIZE CONFIG_NAOS_TRACE_BUF_SIZE

#define NAOS_TRACE_REC_SWITCH 1  // TYPE(1) TS(4) CORE(1) ID(1) = 7
#define NAOS_TRACE_REC_EVENT 2   // TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) = 9
#define NAOS_TRACE_REC_BEGIN 3   // TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) ID(1) = 10
#define NAOS_TRACE_REC_END 4     // TYPE(1) TS(4) ID(1) = 6
#define NAOS_TRACE_REC_VALUE 5   // TYPE(1) TS(4) CAT(1) NAME(1) VAL(4) = 11
#define NAOS_TRACE_REC_LABEL 6   // TYPE(1) ID(1) TEXT(*) NUL(1) = 3+
#define NAOS_TRACE_REC_TASK 7    // TYPE(1) ID(1) NAME(*) NUL(1) = 3+

typedef enum {
  NAOS_TRACE_CMD_START,
  NAOS_TRACE_CMD_STOP,
  NAOS_TRACE_CMD_READ,
  NAOS_TRACE_CMD_STATUS,
} naos_trace_cmd_t;

static portMUX_TYPE naos_trace_spinlock = portMUX_INITIALIZER_UNLOCKED;
static bool naos_trace_active = false;
static int64_t naos_trace_start_time = 0;

// byte ring buffer; records never wrap, zeros pad to buffer start
static uint8_t *naos_trace_buffer = NULL;
static uint32_t naos_trace_capacity = 0;
static volatile uint32_t naos_trace_head = 0;
static volatile uint32_t naos_trace_tail = 0;
static volatile uint32_t naos_trace_used = 0;
static volatile uint32_t naos_trace_dropped = 0;

// task handle-to-id mapping for dedup
static TaskHandle_t naos_trace_tasks[NAOS_TRACE_MAX_TASKS] = {0};
static volatile uint16_t naos_trace_task_count = 0;

// label string-to-id mapping for dedup (by pointer identity)
static const char *naos_trace_labels[NAOS_TRACE_MAX_LABELS] = {0};
static volatile uint8_t naos_trace_label_count = 0;

// auto-incrementing span instance counter
static volatile uint8_t naos_trace_span_id = 0;

// per-core last task for change detection
static TaskHandle_t naos_trace_last_task[portNUM_PROCESSORS] = {0};

static void naos_trace_write(const uint8_t *data, size_t len) {
  // must be called with spinlock held

  // determine buffer space
  uint32_t free_bytes = naos_trace_capacity - naos_trace_used;
  uint32_t end_space = naos_trace_capacity - naos_trace_head;

  if (end_space >= len) {
    // record fits at current position
    if (free_bytes < len) {
      naos_trace_dropped++;
      return;
    }
    memcpy(naos_trace_buffer + naos_trace_head, data, len);
    naos_trace_head += len;
    if (naos_trace_head == naos_trace_capacity) {
      naos_trace_head = 0;
    }
    naos_trace_used += len;
  } else {
    // pad remainder and write at start
    uint32_t total = end_space + len;
    if (free_bytes < total) {
      naos_trace_dropped++;
      return;
    }
    memset(naos_trace_buffer + naos_trace_head, 0, end_space);
    memcpy(naos_trace_buffer, data, len);
    naos_trace_head = len;
    naos_trace_used += total;
  }
}

static size_t naos_trace_record_size(const uint8_t *data, size_t available) {
  // determine record size from type byte
  if (available == 0) {
    return 0;
  }
  switch (data[0]) {
    case NAOS_TRACE_REC_SWITCH:
      return 7;
    case NAOS_TRACE_REC_EVENT:
      return 9;
    case NAOS_TRACE_REC_BEGIN:
      return 10;
    case NAOS_TRACE_REC_END:
      return 6;
    case NAOS_TRACE_REC_VALUE:
      return 11;
    case NAOS_TRACE_REC_LABEL:
    case NAOS_TRACE_REC_TASK:
      for (size_t i = 2; i < available; i++) {
        if (data[i] == 0) {
          return i + 1;
        }
      }
      return 0;
    default:
      return 0;
  }
}

static uint8_t naos_trace_find_task(TaskHandle_t handle) {
  // must be called with spinlock held

  // search existing
  for (uint16_t i = 0; i < naos_trace_task_count; i++) {
    if (naos_trace_tasks[i] == handle) {
      return i;
    }
  }

  // add new if space available
  if (naos_trace_task_count < NAOS_TRACE_MAX_TASKS) {
    uint8_t id = (uint8_t)naos_trace_task_count;
    naos_trace_tasks[id] = handle;
    naos_trace_task_count++;

    // write TASK record
    const char *name = pcTaskGetName(handle);
    if (!name) {
      name = "?";
    }
    size_t name_len = strlen(name);
    if (name_len > 15) {
      name_len = 15;
    }
    uint8_t rec[2 + 15 + 1];
    rec[0] = NAOS_TRACE_REC_TASK;
    rec[1] = id;
    memcpy(rec + 2, name, name_len);
    rec[2 + name_len] = 0;
    naos_trace_write(rec, 2 + name_len + 1);

    return id;
  }

  return UINT8_MAX;
}

static int naos_trace_find_label(const char *text) {
  // must be called with spinlock held

  // search existing by pointer identity
  for (uint8_t i = 0; i < naos_trace_label_count; i++) {
    if (naos_trace_labels[i] == text) {
      return i;
    }
  }

  // add new if space available
  if (naos_trace_label_count < NAOS_TRACE_MAX_LABELS) {
    uint8_t id = naos_trace_label_count;
    naos_trace_labels[id] = text;
    naos_trace_label_count++;

    // write LABEL record
    size_t text_len = strlen(text);
    if (text_len > 32) {
      text_len = 32;
    }
    uint8_t rec[2 + 32 + 1];
    rec[0] = NAOS_TRACE_REC_LABEL;
    rec[1] = id;
    memcpy(rec + 2, text, text_len);
    rec[2 + text_len] = 0;
    naos_trace_write(rec, 2 + text_len + 1);

    return id;
  }

  return -1;
}

void IRAM_ATTR naos_trace_task_switched_in(void *task) {
  // check if trace is active
  if (!naos_trace_active) {
    return;
  }

  // skip if same task as last time on this core
  uint8_t core = (uint8_t)xPortGetCoreID();
  if ((TaskHandle_t)task == naos_trace_last_task[core]) {
    return;
  }

  // update last task record
  naos_trace_last_task[core] = (TaskHandle_t)task;

  // find or register task and write SWITCH record
  portENTER_CRITICAL_ISR(&naos_trace_spinlock);
  uint8_t id = naos_trace_find_task(task);
  if (id != UINT8_MAX) {
    uint32_t ts = (uint32_t)(esp_timer_get_time() - naos_trace_start_time);
    uint8_t rec[7] = {NAOS_TRACE_REC_SWITCH};
    memcpy(rec + 1, &ts, 4);
    rec[5] = core;
    rec[6] = id;
    naos_trace_write(rec, 7);
  }
  portEXIT_CRITICAL_ISR(&naos_trace_spinlock);
}

static naos_msg_reply_t naos_trace_handle_start(naos_msg_t msg) {
  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // reset and activate
  portENTER_CRITICAL(&naos_trace_spinlock);
  naos_trace_active = false;
  naos_trace_head = 0;
  naos_trace_tail = 0;
  naos_trace_used = 0;
  naos_trace_dropped = 0;
  naos_trace_task_count = 0;
  naos_trace_label_count = 0;
  naos_trace_span_id = 0;
  memset(naos_trace_last_task, 0, sizeof(naos_trace_last_task));
  memset(naos_trace_tasks, 0, sizeof(naos_trace_tasks));
  memset(naos_trace_labels, 0, sizeof(naos_trace_labels));
  naos_trace_start_time = esp_timer_get_time();
  naos_trace_active = true;
  portEXIT_CRITICAL(&naos_trace_spinlock);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_trace_handle_stop(naos_msg_t msg) {
  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // deactivate
  portENTER_CRITICAL(&naos_trace_spinlock);
  naos_trace_active = false;
  portEXIT_CRITICAL(&naos_trace_spinlock);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_trace_handle_read(naos_msg_t msg) {
  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // get MTU
  size_t mtu = naos_msg_get_mtu(msg.session);

  // allocate chunk buffer
  uint8_t *chunk = malloc(mtu);
  if (chunk == NULL) {
    return NAOS_MSG_ERROR;
  }

  // snapshot buffer state
  portENTER_CRITICAL(&naos_trace_spinlock);
  uint32_t total = naos_trace_used;
  uint32_t pos = naos_trace_tail;
  portEXIT_CRITICAL(&naos_trace_spinlock);
  uint32_t remaining = total;

  // stream records in MTU-sized chunks
  while (remaining > 0) {
    size_t chunk_len = 0;
    uint32_t consumed = 0;

    while (consumed < remaining) {
      // skip padding
      if (naos_trace_buffer[pos] == 0) {
        uint32_t pad = naos_trace_capacity - pos;
        pos = 0;
        consumed += pad;
        continue;
      }

      // determine record size
      size_t available = naos_trace_capacity - pos;
      size_t rec_size = naos_trace_record_size(naos_trace_buffer + pos, available);
      if (rec_size == 0) {
        consumed = remaining;
        break;
      }

      // stop if record doesn't fit in chunk
      if (chunk_len + rec_size > mtu) {
        break;
      }

      // copy record to chunk
      memcpy(chunk + chunk_len, naos_trace_buffer + pos, rec_size);
      chunk_len += rec_size;
      pos += rec_size;
      if (pos == naos_trace_capacity) {
        pos = 0;
      }
      consumed += rec_size;
    }

    remaining -= consumed;

    // advance tail
    portENTER_CRITICAL(&naos_trace_spinlock);
    naos_trace_tail = pos;
    naos_trace_used -= consumed;
    portEXIT_CRITICAL(&naos_trace_spinlock);

    // send chunk
    if (chunk_len > 0) {
      naos_msg_send((naos_msg_t){
          .session = msg.session,
          .endpoint = NAOS_TRACE_ENDPOINT,
          .data = chunk,
          .len = chunk_len,
      });
    }

    // yield
    naos_delay(1);
  }

  free(chunk);

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_trace_handle_status(naos_msg_t msg) {
  // check message
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // snapshot state
  portENTER_CRITICAL(&naos_trace_spinlock);
  bool active = naos_trace_active;
  uint32_t used = naos_trace_used;
  uint32_t dropped = naos_trace_dropped;
  portEXIT_CRITICAL(&naos_trace_spinlock);

  // send status: ACTIVE(1) | BUF_SIZE(4) | BUF_USED(4) | DROPPED(4)
  uint8_t buf[13] = {0};
  buf[0] = active ? 1 : 0;
  memcpy(buf + 1, &naos_trace_capacity, 4);
  memcpy(buf + 5, &used, 4);
  memcpy(buf + 9, &dropped, 4);

  naos_msg_send((naos_msg_t){
      .session = msg.session,
      .endpoint = NAOS_TRACE_ENDPOINT,
      .data = buf,
      .len = sizeof(buf),
  });

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_trace_handle(naos_msg_t msg) {
  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // check lock
  if (naos_msg_is_locked(msg.session)) {
    return NAOS_MSG_LOCKED;
  }

  // get command
  naos_trace_cmd_t cmd = msg.data[0];
  msg.data = &msg.data[1];
  msg.len -= 1;

  // dispatch
  switch (cmd) {
    case NAOS_TRACE_CMD_START:
      return naos_trace_handle_start(msg);
    case NAOS_TRACE_CMD_STOP:
      return naos_trace_handle_stop(msg);
    case NAOS_TRACE_CMD_READ:
      return naos_trace_handle_read(msg);
    case NAOS_TRACE_CMD_STATUS:
      return naos_trace_handle_status(msg);
    default:
      return NAOS_MSG_UNKNOWN;
  }
}

void naos_trace_install() {
  // allocate buffer
  naos_trace_capacity = NAOS_TRACE_BUF_SIZE;
  naos_trace_buffer = calloc(1, naos_trace_capacity);
  if (naos_trace_buffer == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_TRACE_ENDPOINT,
      .name = "trace",
      .handle = naos_trace_handle,
  });
}

void naos_trace_event(const char *category, const char *name, uint16_t arg) {
  // check state
  if (!naos_trace_active) {
    return;
  }

  // get timestamp
  uint32_t ts = (uint32_t)(esp_timer_get_time() - naos_trace_start_time);

  portENTER_CRITICAL(&naos_trace_spinlock);

  // resolve labels
  int cat_id = naos_trace_find_label(category);
  int name_id = naos_trace_find_label(name);

  // write EVENT record
  if (cat_id >= 0 && name_id >= 0) {
    uint8_t rec[9] = {NAOS_TRACE_REC_EVENT};
    memcpy(rec + 1, &ts, 4);
    rec[5] = (uint8_t)cat_id;
    rec[6] = (uint8_t)name_id;
    memcpy(rec + 7, &arg, 2);
    naos_trace_write(rec, 9);
  }

  portEXIT_CRITICAL(&naos_trace_spinlock);
}

void naos_trace_value(const char *category, const char *name, int32_t value) {
  // check state
  if (!naos_trace_active) {
    return;
  }

  // get timestamp
  uint32_t ts = (uint32_t)(esp_timer_get_time() - naos_trace_start_time);

  portENTER_CRITICAL(&naos_trace_spinlock);

  // resolve labels
  int cat_id = naos_trace_find_label(category);
  int name_id = naos_trace_find_label(name);

  // write VALUE record
  if (cat_id >= 0 && name_id >= 0) {
    uint8_t rec[11] = {NAOS_TRACE_REC_VALUE};
    memcpy(rec + 1, &ts, 4);
    rec[5] = (uint8_t)cat_id;
    rec[6] = (uint8_t)name_id;
    memcpy(rec + 7, &value, 4);
    naos_trace_write(rec, 11);
  }

  portEXIT_CRITICAL(&naos_trace_spinlock);
}

int naos_trace_begin(const char *category, const char *name, uint16_t arg) {
  // check state
  if (!naos_trace_active) {
    return -1;
  }

  // get timestamp
  uint32_t ts = (uint32_t)(esp_timer_get_time() - naos_trace_start_time);

  portENTER_CRITICAL(&naos_trace_spinlock);

  // resolve labels
  int cat_id = naos_trace_find_label(category);
  int name_id = naos_trace_find_label(name);

  // write BEGIN record with span instance id
  int span_id = -1;
  if (cat_id >= 0 && name_id >= 0) {
    span_id = naos_trace_span_id++;
    uint8_t rec[10] = {NAOS_TRACE_REC_BEGIN};
    memcpy(rec + 1, &ts, 4);
    rec[5] = (uint8_t)cat_id;
    rec[6] = (uint8_t)name_id;
    memcpy(rec + 7, &arg, 2);
    rec[9] = (uint8_t)span_id;
    naos_trace_write(rec, 10);
  }

  portEXIT_CRITICAL(&naos_trace_spinlock);

  return span_id;
}

void naos_trace_end(int id) {
  // check state
  if (id < 0 || !naos_trace_active) {
    return;
  }

  // get timestamp
  uint32_t ts = (uint32_t)(esp_timer_get_time() - naos_trace_start_time);

  // prepare record
  uint8_t rec[6] = {NAOS_TRACE_REC_END};
  memcpy(rec + 1, &ts, 4);
  rec[5] = (uint8_t)id;

  // write record
  portENTER_CRITICAL(&naos_trace_spinlock);
  naos_trace_write(rec, 6);
  portEXIT_CRITICAL(&naos_trace_spinlock);
}
