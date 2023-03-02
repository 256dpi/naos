#include <naos.h>
#include <naos/sys.h>

#include <esp_freertos_hooks.h>

#if defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_240)
#define NAOS_CPU_MAX_IDLE_CALLS 368000.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_160)
#define NAOS_CPU_MAX_IDLE_CALLS 245333.f
#elif defined(CONFIG_ESP32_DEFAULT_CPU_FREQ_80)
#define NAOS_CPU_MAX_IDLE_CALLS 122666.f
#else
#error "Unsupported CPU Frequency"
#endif

static uint64_t naos_cpu_idle0 = 0;
static uint64_t naos_cpu_idle1 = 0;
static float naos_cpu_usage0 = 0;
static float naos_cpu_usage1 = 0;

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
  naos_cpu_usage0 = (naos_cpu_usage0 * 4 + usage0) / 5;
  naos_cpu_usage1 = (naos_cpu_usage1 * 4 + usage1) / 5;

  // set parameters
  naos_set_d("cpu-usage0", naos_cpu_usage0);
  naos_set_d("cpu-usage1", naos_cpu_usage1);
}

static naos_param_t naos_cpu_params[] = {
    {.name = "cpu-usage0", .type = NAOS_DOUBLE, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
    {.name = "cpu-usage1", .type = NAOS_DOUBLE, .mode = NAOS_VOLATILE | NAOS_SYSTEM | NAOS_LOCKED},
};

void naos_cpu_init() {
  // register parameters
  for (size_t i = 0; i < (sizeof(naos_cpu_params) / sizeof(naos_cpu_params[0])); i++) {
    naos_register(&naos_cpu_params[i]);
  }

  // register hooks
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_cpu_hook0, 0));
  ESP_ERROR_CHECK(esp_register_freertos_idle_hook_for_cpu(naos_cpu_hook1, 1));

  // start update timer
  naos_repeat("naos-cpu", 1000, naos_cpu_update);
}
