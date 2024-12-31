#ifndef NAOS_SERIAL_H
#define NAOS_SERIAL_H

#include <sdkconfig.h>

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
