#ifndef NAOS_SERIAL_H
#define NAOS_SERIAL_H

/**
 * NAOS SERIAL INTERFACE
 * =====================
 *
 * The serial interface is used to communicate with the device over a serial
 * connection. This can either be a separate USB/JTAG based connection or a
 * connection that runs over the standard input/output (UART) of the device
 * and is multiplexed with the system console.
 *
 * The message format for both interfaces is the same and supports hot plugging
 * of the serial connection. All messages are encoded in base64 and prefixed
 * with the magic string `NAOS!`. Each message is terminated with a newline
 * character.
 *
 * Hint: Create a session with "NAOS!AQAAAKq7qrs=" to test a serial connection.
 */

/**
 * Initialize the primary IO based serial configuration.
 *
 * This will use the standard input/output file streams for communication.
 *
 * Notes: Requires that `CONFIG_ESP_CONSOLE` is configured accordingly.
 */
void naos_serial_init_stdio();

/**
 * Initialize the secondary IO based serial configuration.
 *
 * This will open the `/dev/secondary` file stream and use it for communication.
 *
 * Note: Requires that `CONFIG_ESP_CONSOLE_SECONDARY` is configured accordingly.
 */
void naos_serial_init_secio();

/**
 * Initialize the USB based serial configuration.
 *
 * Note: The function will not link if USB/JTAG support is unavailable.
 */
void naos_serial_init_usb();

#endif  // NAOS_SERIAL_H
