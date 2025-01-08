#ifndef NAOS_CONNECT_H
#define NAOS_CONNECT_H

/**
 * NAOS CONNECT SERVICE
 * ====================
 *
 * The NAOS connect service provides a mechanism to establish a connection to a
 * NAOS Hub instance or compatible server over WebSockets. The service will
 * create a reverse messaging channel that the server can use to communicate
 * with the device.
 *
 * The binary message format is a follows:
 * | VERSION (1) | COMMAND (1) | PAYLOAD (*) |
 *
 * The following commands are supported:
 * - NAOS_CONN_MSG [I/O]: A control message.
 *
 * Parameters:
 * - connect-url: The URL of the NAOS Hub.
 * - connect-token: The token to authenticate with the NAOS Hub.
 * - connect-configure: The action to trigger a reconfiguration.
 * - connect-status: The status of the connection.
 */

void naos_connect_init();

#endif  // NAOS_CONNECT_H
