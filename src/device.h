#ifndef _NADK_DEVICE_H
#define _NADK_DEVICE_H

#include <nadk.h>

/**
 * Initialize the device subsystem.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device information.
 */
void nadk_device_init(nadk_device_t* device);

/**
 * Will return the device.
 *
 * @return
 */
nadk_device_t* nadk_device();

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
