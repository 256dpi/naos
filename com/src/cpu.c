#include <naos.h>
#include <naos/sys.h>
#include <naos/metrics.h>

#include <freertos/FreeRTOS.h>
#include <freertos/task.h>

#define URT configRUN_TIME_COUNTER_TYPE

static TaskHandle_t naos_cpu_handles[2] = {0};
static URT naos_cpu_system_run_time = 0;
static URT naos_cpu_task_run_time[2] = {0};
static float naos_cpu_usage[2] = {0};

static void naos_cpu_update() {
  // get total system run time
  URT total_system_rt = portGET_RUN_TIME_COUNTER_VALUE();

  // get total idle run times
  URT total_idle_rt[2] = {0};
  for (size_t i = 0; i < 2; i++) {
    TaskStatus_t status;
    vTaskGetInfo(naos_cpu_handles[i], &status, pdFALSE, eRunning);
    total_idle_rt[i] = status.ulRunTimeCounter;
  }

  // calculate differences
  URT system_rtd = total_system_rt - naos_cpu_system_run_time;
  URT idle_rtd[2] = {0};
  for (size_t i = 0; i < 2; i++) {
    idle_rtd[i] = total_idle_rt[i] - naos_cpu_task_run_time[i];
  }

  // calculate CPU usages
  for (size_t i = 0; i < 2; i++) {
    naos_cpu_usage[i] = 1 - (float)idle_rtd[i] / (float)system_rtd;
  }

  // update values
  naos_cpu_system_run_time = total_system_rt;
  for (size_t i = 0; i < 2; i++) {
    naos_cpu_task_run_time[i] = total_idle_rt[i];
  }
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
  for (size_t i = 0; i < NAOS_COUNT(naos_cpu_metrics); i++) {
    naos_metrics_add(&naos_cpu_metrics[i]);
  }

  // get task handles
  naos_cpu_handles[0] = xTaskGetHandle("IDLE0");
  naos_cpu_handles[1] = xTaskGetHandle("IDLE1");
  if (naos_cpu_handles[0] == NULL || naos_cpu_handles[1] == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
    return;
  }

  // start update timer
  naos_repeat("naos-cpu", 250, naos_cpu_update);
}

void naos_cpu_get(float *cpu0, float *cpu1) {
  // set values
  *cpu0 = naos_cpu_usage[0];
  *cpu1 = naos_cpu_usage[1];
}
