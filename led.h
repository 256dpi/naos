#ifndef _NADK_LED_H
#define _NADK_LED_H

/**
 * Initialize the LED management system.
 *
 * Note: Should only be called once on boot.
 */
void nadk_led_init();

/**
 * Set value of the LEDs.
 *
 * @param red
 * @param green
 */
void nadk_led_set(bool red, bool green);

#endif  // _NADK_LED_H
