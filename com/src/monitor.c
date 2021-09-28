#include <freertos/FreeRTOS.h>

#include <esp_freertos_hooks.h>
#include <freertos/task.h>
#include <naos.h>
#include <sdkconfig.h>

#include "monitor.h"

#if defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_240)
#define NAOS_MAX_IDLE_CALLS 368000.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_160)
#define NAOS_MAX_IDLE_CALLS 245333.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_80)
#define NAOS_MAX_IDLE_CALLS 122666.f
#else
#error "Unsupported CPU Frequency"
#endif

static uint64_t naos_idle0_calls = 0;
static uint64_t naos_idle1_calls = 0;

static float naos_cpu_usage0 = 0;
static float naos_cpu_usage1 = 0;

static bool naos_idle_hook0() {
  naos_idle0_calls++;
  return false;
}

static bool naos_idle_hook1() {
  naos_idle1_calls++;
  return false;
}

static void naos_monitor_task() {
  for (;;) {
    // get counters
    float idle0 = (float)naos_idle0_calls;
    float idle1 = (float)naos_idle1_calls;

    // reset counters
    naos_idle0_calls = 0;
    naos_idle1_calls = 0;

    // clamp counters
    if (idle0 > NAOS_MAX_IDLE_CALLS) {
      idle0 = NAOS_MAX_IDLE_CALLS;
    }
    if (idle1 > NAOS_MAX_IDLE_CALLS) {
      idle1 = NAOS_MAX_IDLE_CALLS;
    }

    // calculate usage
    naos_cpu_usage0 = 1.f - idle0 / NAOS_MAX_IDLE_CALLS;
    naos_cpu_usage1 = 1.f - idle1 / NAOS_MAX_IDLE_CALLS;

    // wait a second
    naos_delay(1000);
  }
}

void naos_monitor_init() {
  // register hooks
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_idle_hook0, 0));
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_idle_hook1, 1));

  // run task
  xTaskCreatePinnedToCore(naos_monitor_task, "naos-monitor", 2048, NULL, 1, NULL, 1);
}

naos_cpu_usage_t naos_monitor_get() {
  // prepare usage
  naos_cpu_usage_t usage = {
      .cpu0 = naos_cpu_usage0,
      .cpu1 = naos_cpu_usage1,
  };

  return usage;
}
