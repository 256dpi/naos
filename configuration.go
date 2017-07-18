package naos

import (
	"time"

	"github.com/shiftr-io/naos/ble"
)

// Scan will scan for available Bluetooth based device configurations.
func Scan(duration time.Duration) (map[string]*ble.Configuration, error) {
	// initialize ble
	err := ble.Initialize()
	if err != nil {
		return nil, err
	}

	// scan for addresses
	addresses, err := ble.Scan(duration)
	if err != nil {
		return nil, err
	}

	// prepare map
	var store map[string]*ble.Configuration

	// explore devices
	for _, a := range addresses {
		// connect to device
		dev, err := ble.Connect(a)
		if err != nil {
			return nil, err
		}

		// read configuration
		conf, err := ble.ReadConfiguration(dev)
		if err != nil {
			return nil, err
		}

		// assign configuration to address
		store[dev.Address()] = conf

		// close connection
		err = dev.Close()
		if err != nil {
			return nil, err
		}
	}

	return store, nil
}
