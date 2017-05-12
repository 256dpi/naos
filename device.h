#ifndef _NADK_DEVICE_H
#define _NADK_DEVICE_H

/**
 * Initialize the device management system.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device to be managed.
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
 * @param topic
 * @param payload
 * @param len
 */
void nadk_device_forward(const char* topic, const char* payload, unsigned int len);

#endif  // _NADK_DEVICE_H
