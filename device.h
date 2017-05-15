#ifndef _NADK_DEVICE_H
#define _NADK_DEVICE_H

/**
 * Initialize the device management system.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device information.
 */
void nadk_device_init(nadk_device_t* device);

/**
 * Start the device process.
 */
void nadk_device_start();

/**
 * Stop the device process.
 */
void nadk_device_stop();

/**
 * Forward a message to the device process.
 *
 * @param topic - The topic.
 * @param payload - The payload.
 * @param len - The payload length.
 * @param scope - The scope.
 */
void nadk_device_forward(const char* topic, const char* payload, unsigned int len, nadk_scope_t scope);

#endif  // _NADK_DEVICE_H
