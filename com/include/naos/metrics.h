#ifndef NAOS_METRICS_H
#define NAOS_METRICS_H

#include <stddef.h>

/**
 * NAOS METRICS SYSTEM
 * ===================
 *
 * The metrics system provides a mechanism for collecting device metrics.
 * Metrics are defined by a name, kind, and type. Supported are counters and
 * gauges, with long and double values as types. Metrics can be collected via
 * the exposed control plane endpoint.
 *
 * Metrics define either a single scalar value or a set of values. The set
 * is defined by a list of keys with a corresponding list of values. That way
 * multidimensional metrics can be defined in a space-efficient way.
 *
 * The following is an example of a scalar metric definition:
 *
 *  static double temperature = 0.0;
 *  static naos_metric_t metric = {
 *    .name = "temperature",
 *    .kind = NAOS_METRIC_GAUGE,
 *    .type = NAOS_METRIC_DOUBLE,
 *    .data = &temperature,
 *  };
 *
 *  The following is an example of a multi-dimensional metric definition:
 *
 *  static long transferred_bytes[2][2] = {0};
 *  naos_metric_t metric = {
 *    .name = "transferred_bytes",
 *    .kind = NAOS_METRIC_COUNTER,
 *    .type = NAOS_METRIC_LONG,
 *    .data = transferred_bytes,
 *    .keys = {"interface", "direction"},
 *    .values = {
 *      "eth", "wifi", NULL
 *      "rx", "tx"
 *    },
 *  }
 */

#define NAOS_METRIC_KEYS 4
#define NAOS_METRIC_VALUES 16

typedef enum {
  NAOS_METRIC_COUNTER = 0,
  NAOS_METRIC_GAUGE,
} naos_metric_kind_t;

typedef enum {
  NAOS_METRIC_LONG = 0,
  NAOS_METRIC_FLOAT,
  NAOS_METRIC_DOUBLE,
} naos_metric_type_t;

typedef struct {
  const char *name;
  naos_metric_kind_t kind;
  naos_metric_type_t type;
  void * data;
  const char *keys[NAOS_METRIC_KEYS + 1];
  const char *values[NAOS_METRIC_VALUES + NAOS_METRIC_KEYS];
  // internal
  int num_keys;
  int num_values[NAOS_METRIC_KEYS];
  int first_value[NAOS_METRIC_KEYS];
  size_t size;
} naos_metric_t;

void naos_metrics_add(naos_metric_t * metric);

#endif // NAOS_METRICS_H
