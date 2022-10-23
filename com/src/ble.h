#ifndef _NAOS_BLE_H
#define _NAOS_BLE_H

/**
 * Initialize the BLE subsystem.
 *
 * @note Should only be called once on boot.
 *
 * @param rcb The read callback.
 * @param wcb The write callback.
 */
void naos_ble_init();

#endif  // _NAOS_BLE_H
