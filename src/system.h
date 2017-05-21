#ifndef _NADK_SYSTEM_H
#define _NADK_SYSTEM_H

/**
 * Initialize the main system.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device information.
 */
void nadk_system_init(nadk_device_t *device);

#endif  // _NADK_SYSTEM_H
