package ble

import (
	"time"

	"tinygo.org/x/bluetooth"
)

func write(characteristic bluetooth.DeviceCharacteristic, value []byte) error {
	_, err := characteristic.WriteWithoutResponse(value)
	time.Sleep(5 * time.Millisecond)
	return err
}
