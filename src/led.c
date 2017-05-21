#include <driver/ledc.h>
#include <freertos/FreeRTOS.h>
#include <freertos/semphr.h>

#include "general.h"

static SemaphoreHandle_t nadk_led_mutex;

#define NADK_LED_RED_PIN 18
#define NADK_LED_GREEN_PIN 19

void nadk_led_set(bool red, bool green) {
  // acquire mutex
  NADK_LOCK(nadk_led_mutex);

  // set duties
  ESP_ERROR_CHECK(ledc_set_duty(LEDC_HIGH_SPEED_MODE, LEDC_CHANNEL_6, red ? 512 : 0));
  ESP_ERROR_CHECK(ledc_set_duty(LEDC_HIGH_SPEED_MODE, LEDC_CHANNEL_7, green ? 512 : 0));

  // update channels
  ESP_ERROR_CHECK(ledc_update_duty(LEDC_HIGH_SPEED_MODE, LEDC_CHANNEL_6));
  ESP_ERROR_CHECK(ledc_update_duty(LEDC_HIGH_SPEED_MODE, LEDC_CHANNEL_7));

  // release mutex
  NADK_UNLOCK(nadk_led_mutex);
}

void nadk_led_init() {
  // create mutex
  nadk_led_mutex = xSemaphoreCreateMutex();

  // prepare ledc timer config
  ledc_timer_config_t t = {
      .bit_num = LEDC_TIMER_10_BIT, .freq_hz = 5000, .speed_mode = LEDC_HIGH_SPEED_MODE, .timer_num = LEDC_TIMER_3};

  // configure ledc timer
  ESP_ERROR_CHECK(ledc_timer_config(&t));

  // prepare ledc channel config
  ledc_channel_config_t c = {
      .duty = 0, .intr_type = LEDC_INTR_DISABLE, .speed_mode = LEDC_HIGH_SPEED_MODE, .timer_sel = LEDC_TIMER_3};

  // configure red led
  c.gpio_num = NADK_LED_RED_PIN;
  c.channel = LEDC_CHANNEL_6;
  ESP_ERROR_CHECK(ledc_channel_config(&c));

  // configure green led
  c.gpio_num = NADK_LED_GREEN_PIN;
  c.channel = LEDC_CHANNEL_7;
  ESP_ERROR_CHECK(ledc_channel_config(&c));
}
