#ifndef _NADK_MANAGER_H
#define _NADK_MANAGER_H

/**
 * Initialize the main management system.
 *
 * Note: Should only be called once on boot.
 *
 * @param device - The device to be managed.
 */
void nadk_manager_init(nadk_device_t* device);

#endif  // _NADK_MANAGER_H
