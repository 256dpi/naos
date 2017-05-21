#ifndef _NADK_MANAGER_H
#define _NADK_MANAGER_H

/**
 * Initialize the system management system.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device information.
 */
void nadk_system_init(nadk_device_t *device);

#endif  // _NADK_MANAGER_H
