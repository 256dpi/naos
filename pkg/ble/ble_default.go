//go:build !linux

package ble

import "tinygo.org/x/bluetooth"

func write(characteristic bluetooth.DeviceCharacteristic, value []byte) error {
	_, err := characteristic.Write(value)
	return err
}
