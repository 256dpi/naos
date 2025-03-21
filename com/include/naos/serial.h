#ifndef NAOS_SERIAL_H
#define NAOS_SERIAL_H

#include <sdkconfig.h>

/**
 * NAOS SERIAL INTERFACE
 * =====================
 *
 * The serial interface is used to communicate with the device over a serial
 * connection. This can either be a separate USB (CDC) based connection or a
 * connection that runs over the standard input/output (UART) of the device
 * and is multiplexed with the system console.
 *
 * The message format for both interfaces is the same and supports hot plugging
 * of the serial connection. All messages are encoded in base64 and prefixed
 * with the magic string `NAOS!`. Each message is terminated with a newline
 * character.
 */

/**
 * Initialize the STDIO based serial configuration.
 */
void naos_serial_init_stdio();

/**
 * Whether USB based serial configuration is available.
 */
#if CONFIG_SOC_USB_OTG_SUPPORTED
#define NAOS_SERIAL_USB_AVAILABLE 1
#else
#define NAOS_SERIAL_USB_AVAILABLE 0
#endif

/**
 * Initialize the USB based serial configuration.
 */
void naos_serial_init_usb();

#endif  // NAOS_SERIAL_H
