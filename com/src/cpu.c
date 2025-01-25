#include <naos.h>
#include <naos/sys.h>
#include <naos/metrics.h>

#include <esp_freertos_hooks.h>

// TODO: Use ulTaskGetIdleRunTimePercent() instead of hooks when FreeRTOS is
//  updated to version 10.5.1 or later.

#if defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_240) || defined(CONFIG_ESP32S3_DEFAULT_CPU_FREQ_240)
#define NAOS_CPU_MAX_IDLE_CALLS 368000.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_160) || defined(CONFIG_ESP32S3_DEFAULT_CPU_FREQ_160)
#define NAOS_CPU_MAX_IDLE_CALLS 245333.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_80) || defined(CONFIG_ESP32S3_DEFAULT_CPU_FREQ_80)
#define NAOS_CPU_MAX_IDLE_CALLS 122666.f
#else
#error "Unsupported CPU Frequency"
#endif

static uint64_t naos_cpu_idle0 = 0;
static uint64_t naos_cpu_idle1 = 0;
static float naos_cpu_usage[2] = {0};

static bool naos_cpu_hook0() {
  naos_cpu_idle0++;
  return false;
}

static bool naos_cpu_hook1() {
  naos_cpu_idle1++;
  return false;
}

static void naos_cpu_update() {
  // get counters
  float idle0 = (float)naos_cpu_idle0;
  float idle1 = (float)naos_cpu_idle1;

  // reset counters
  naos_cpu_idle0 = 0;
  naos_cpu_idle1 = 0;

  // clamp counters
  if (idle0 > NAOS_CPU_MAX_IDLE_CALLS) {
    idle0 = NAOS_CPU_MAX_IDLE_CALLS;
  }
  if (idle1 > NAOS_CPU_MAX_IDLE_CALLS) {
    idle1 = NAOS_CPU_MAX_IDLE_CALLS;
  }

  // calculate new usage
  float usage0 = 1.f - idle0 / NAOS_CPU_MAX_IDLE_CALLS;
  float usage1 = 1.f - idle1 / NAOS_CPU_MAX_IDLE_CALLS;

  // add to usage
  naos_cpu_usage[0] = (naos_cpu_usage[0] * 4 + usage0) / 5;
  naos_cpu_usage[1] = (naos_cpu_usage[1] * 4 + usage1) / 5;
}

static naos_metric_t naos_cpu_metrics[] = {{
    .name = "cpu-usage",
    .kind = NAOS_METRIC_GAUGE,
    .type = NAOS_METRIC_FLOAT,
    .data = naos_cpu_usage,
    .keys = {"cpu"},
    .values = {"0", "1"},
}};

void naos_cpu_init() {
  // add metrics
  for (size_t i = 0; i < sizeof(naos_cpu_metrics) / sizeof(naos_metric_t); i++) {
    naos_metric_t *metric = &naos_cpu_metrics[i];
    naos_metrics_add(metric);
  }

  // register hooks
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_cpu_hook0, 0));
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_cpu_hook1, 1));

  // start update timer
  naos_repeat("naos-cpu", 1000, naos_cpu_update);
}

void naos_cpu_get(float *cpu0, float *cpu1) {
  // set values
  *cpu0 = naos_cpu_usage[0];
  *cpu1 = naos_cpu_usage[1];
}
