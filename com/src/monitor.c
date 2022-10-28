#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <freertos/timers.h>
#include <esp_freertos_hooks.h>

#include <naos.h>

#if defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_240)
#define NAOS_MAX_IDLE_CALLS 368000.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_160)
#define NAOS_MAX_IDLE_CALLS 245333.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_80)
#define NAOS_MAX_IDLE_CALLS 122666.f
#else
#error "Unsupported CPU Frequency"
#endif

static uint64_t naos_monitor_idle0 = 0;
static uint64_t naos_monitor_idle1 = 0;

static bool naos_monitor_hook0() {
  naos_monitor_idle0++;
  return false;
}

static bool naos_monitor_hook1() {
  naos_monitor_idle1++;
  return false;
}

static void naos_monitor_update() {
  // get counters
  float idle0 = (float)naos_monitor_idle0;
  float idle1 = (float)naos_monitor_idle1;

  // reset counters
  naos_monitor_idle0 = 0;
  naos_monitor_idle1 = 0;

  // clamp counters
  if (idle0 > NAOS_MAX_IDLE_CALLS) {
    idle0 = NAOS_MAX_IDLE_CALLS;
  }
  if (idle1 > NAOS_MAX_IDLE_CALLS) {
    idle1 = NAOS_MAX_IDLE_CALLS;
  }

  // TODO: Calculate drift?

  // calculate usage
  float cpu_usage0 = 1.f - idle0 / NAOS_MAX_IDLE_CALLS;
  float cpu_usage1 = 1.f - idle1 / NAOS_MAX_IDLE_CALLS;

  // add to usage
  float naos_monitor_cpu0 = (naos_monitor_cpu0 * 4 + cpu_usage0) / 5;
  float naos_monitor_cpu1 = (naos_monitor_cpu1 * 4 + cpu_usage1) / 5;

  // set parameters
  naos_set_d("monitor-cpu0", naos_monitor_cpu0);
  naos_set_d("monitor-cpu1", naos_monitor_cpu1);
}

static naos_param_t naos_monitor_params[] = {
    {.name = "monitor-cpu0", .type = NAOS_DOUBLE, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "monitor-cpu1", .type = NAOS_DOUBLE, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
};

void naos_monitor_init() {
  // register parameters
  for (size_t i = 0; i < (sizeof(naos_monitor_params) / sizeof(naos_monitor_params[0])); i++) {
    naos_register(&naos_monitor_params[i]);
  }

  // register hooks
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_monitor_hook0, 0));
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_monitor_hook1, 1));

  // start update timer
  TimerHandle_t timer = xTimerCreate("naos-monitor", pdMS_TO_TICKS(1000), pdTRUE, 0, naos_monitor_update);
  xTimerStart(timer, 0);
}
