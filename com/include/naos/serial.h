#ifndef NAOS_SERIAL_H
#define NAOS_SERIAL_H

/**
 * NAOS SERIAL INTERFACE
 * =====================
 *
 * The serial interface is used to communicate with the device over a serial
 * connection. Applications can use the primary STDIO, secondary IO, blocking
 * UART or blocking USB/Serial/JTAG based interfaces. All interfaces support
 * multiplexing with the system console.
 *
 * The message format for all interfaces is the same and supports hot plugging
 * of the serial connection. All messages are encoded in base64 and prefixed
 * with the magic string `NAOS!`. Each message begins and ends with a newline
 * character.
 *
 * Hint: Create a session with "NAOS!AQAAAKq7qrs=" to test a serial connection.
 */

/**
 * Initialize the STDIO based serial messaging.
 *
 * This will use the standard input/output file streams for communication.
 *
 * Notes: Requires that `CONFIG_ESP_CONSOLE` is configured accordingly.
 */
void naos_serial_init_stdio();

/**
 * Initialize blocking UART based STDIO serial messaging.
 *
 * Same as `naos_serial_init_stdio`, but configures the UART driver to enable
 * blocking reads.
 */
void naos_serial_init_stdio_uart();

/**
 * Initialize the secondary-IO based serial messaging.
 *
 * This will use the `/dev/secondary` file stream for communication.
 *
 * Note: Requires that `CONFIG_ESP_CONSOLE_SECONDARY` is configured accordingly.
 */
void naos_serial_init_secio();

/**
 * Initialize blocking secondary-IO USB/Serial/JTAG based serial messaging.
 *
 * Same as `naos_serial_init_secio`, but configures the USB/Serial/JTAG driver
 * to enable blocking reads.
 *
 * Note: The function will not link if USB/JTAG support is unavailable.
 */
void naos_serial_init_secio_usj();

/**
 * Initialize blocking USB/Serial/JTAG based serial messaging.
 *
 * This will use the USB/Serial/JTAG peripheral directly for communication.
 *
 * Note: The function will not link if USB/JTAG support is unavailable.
 */
void naos_serial_init_usj();

#endif  // NAOS_SERIAL_H
