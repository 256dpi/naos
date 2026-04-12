//go:build !linux

package ble

import "github.com/256dpi/bluetooth"

func write(characteristic bluetooth.DeviceCharacteristic, value []byte) error {
	_, err := characteristic.Write(value)
	return err
}
