#include <naos/metrics.h>
#include <naos/msg.h>
#include <esp_err.h>
#include <string.h>

#define NAOS_METRICS_NUM 8
#define NAOS_METRICS_ENDPOINT 0x5

typedef enum {
  NAOS_METRICS_CMD_LIST,
  NAOS_METRICS_CMD_DESCRIBE,
  NAOS_METRICS_CMD_READ,
} naos_metrics_cmd_t;

static naos_metric_t *naos_metrics_list[NAOS_METRICS_NUM] = {0};
static size_t naos_metrics_count = 0;

static naos_msg_reply_t naos_metrics_handle_list(naos_msg_t msg) {
  // check length
  if (msg.len != 0) {
    return NAOS_MSG_INVALID;
  }

  // reply structure
  // REF (1) | KIND (1) | TYPE (1) | SIZE(1) | NAME (*)

  // iterate metrics
  uint8_t data[256] = {0};
  for (int i = 0; i < naos_metrics_count; i++) {
    // get metric
    naos_metric_t *metric = naos_metrics_list[i];

    // prepare data
    data[0] = i;
    data[1] = (uint8_t)metric->kind;
    data[2] = (uint8_t)metric->type;
    data[3] = metric->size;
    strcpy((char *)data + 4, metric->name);

    // prepare reply
    naos_msg_t reply = {
        .session = msg.session,
        .endpoint = NAOS_METRICS_ENDPOINT,
        .data = data,
        .len = 4 + strlen(metric->name),
    };

    // send reply
    naos_msg_send(reply);
  }

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_metrics_handle_describe(naos_msg_t msg) {
  // command structure:
  // REF (1)

  // check length
  if (msg.len != 1) {
    return NAOS_MSG_INVALID;
  }

  // check ref
  if (msg.data[0] >= naos_metrics_count) {
    return NAOS_MSG_ERROR;
  }

  // get metric
  naos_metric_t *metric = naos_metrics_list[msg.data[0]];

  // reply structure:
  // 0 | KEY(1) | STRING(*)
  // 1 | KEY(1) | VALUE(1) | STRING(*)

  // iterate keys
  uint8_t data[256] = {0};
  for (int i = 0; i < metric->num_keys; i++) {
    // prepare data
    data[0] = 0;
    data[1] = i;
    strcpy((char *)data + 2, metric->keys[i]);

    // prepare reply
    naos_msg_t reply = {
        .session = msg.session,
        .endpoint = NAOS_METRICS_ENDPOINT,
        .data = data,
        .len = 2 + strlen(metric->keys[i]),
    };

    // send reply
    naos_msg_send(reply);

    // iterate values
    for (int j = 0; j < metric->num_values[i]; j++) {
      // prepare data
      data[0] = 1;
      data[1] = i;
      data[2] = j;
      strcpy((char *)data + 3, metric->values[metric->first_value[i] + j]);

      // prepare reply
      reply = (naos_msg_t){
          .session = msg.session,
          .endpoint = NAOS_METRICS_ENDPOINT,
          .data = data,
          .len = 3 + strlen(metric->values[metric->first_value[i] + j]),
      };

      // send reply
      naos_msg_send(reply);
    }
  }

  return NAOS_MSG_ACK;
}

static naos_msg_reply_t naos_metrics_handle_read(naos_msg_t msg) {
  // command structure:
  // REF (1)

  // check length
  if (msg.len != 1) {
    return NAOS_MSG_INVALID;
  }

  // check ref
  if (msg.data[0] >= naos_metrics_count) {
    return NAOS_MSG_ERROR;
  }

  // get metric
  naos_metric_t *metric = naos_metrics_list[msg.data[0]];

  // determine width
  size_t width = 0;
  if (metric->type == NAOS_METRIC_LONG) {
    width = sizeof(int32_t);
  } else if (metric->type == NAOS_METRIC_FLOAT) {
    width = sizeof(float);
  } else if (metric->type == NAOS_METRIC_DOUBLE) {
    width = sizeof(double);
  }

  // reply structure:
  // DATA(*)

  // prepare reply
  naos_msg_t reply = {
      .session = msg.session,
      .endpoint = NAOS_METRICS_ENDPOINT,
      .data = (uint8_t *)metric->data,
      .len = metric->size * width,
  };

  // send reply
  naos_msg_send(reply);

  return NAOS_MSG_OK;
}

static naos_msg_reply_t naos_metrics_process(naos_msg_t msg) {
  // message structure
  // CMD (1) | *

  // check length
  if (msg.len == 0) {
    return NAOS_MSG_INVALID;
  }

  // check lock status
  if (naos_msg_is_locked(msg.session)) {
    return NAOS_MSG_LOCKED;
  }

  // pluck command
  naos_metrics_cmd_t cmd = msg.data[0];
  msg.data++;
  msg.len--;

  // handle command
  naos_msg_reply_t reply;
  switch (cmd) {
    case NAOS_METRICS_CMD_LIST:
      reply = naos_metrics_handle_list(msg);
      break;
    case NAOS_METRICS_CMD_DESCRIBE:
      reply = naos_metrics_handle_describe(msg);
      break;
    case NAOS_METRICS_CMD_READ:
      reply = naos_metrics_handle_read(msg);
      break;
    default:
      reply = NAOS_MSG_UNKNOWN;
  }

  return reply;
}

void naos_metrics_init() {
  // install endpoint
  naos_msg_install((naos_msg_endpoint_t){
      .ref = NAOS_METRICS_ENDPOINT,
      .name = "metrics",
      .handle = naos_metrics_process,
  });
}

void naos_metrics_add(naos_metric_t *metric) {
  // check space
  if (naos_metrics_count >= NAOS_METRICS_NUM) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // clear internal state
  metric->num_keys = 0;
  memset(metric->num_values, 0, sizeof(metric->num_values));
  memset(metric->first_value, 0, sizeof(metric->first_value));
  metric->size = 1;

  // count keys
  while (metric->keys[metric->num_keys] != NULL) {
    metric->num_keys++;
  }
  if (metric->num_keys > NAOS_METRIC_KEYS) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // count values and find first values
  for (int i = 0; i < metric->num_keys; i++) {
    // set first value start
    if (i == 0) {
      metric->first_value[0] = 0;
    } else {
      metric->first_value[i] = metric->first_value[i - 1] + metric->num_values[i - 1] + 1;
    }

    // count values
    const char **values = metric->values + metric->first_value[i];
    while (values[metric->num_values[i]] != NULL) {
      metric->num_values[i]++;
    }
  }

  // calculate size
  for (int i = 0; i < metric->num_keys; i++) {
    if (i == 0) {
      metric->size = metric->num_values[i];
    } else {
      metric->size *= metric->num_values[i];
    }
  }

  // store metric
  naos_metrics_list[naos_metrics_count] = metric;
  naos_metrics_count++;
}
