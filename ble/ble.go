// Package ble provides a low-level implementation of the NAOS BLE protocol.
package ble

import (
	"errors"
	"runtime"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
	"github.com/go-ble/ble/linux"
)

// Available returns true if BLE is available on the current platform.
func Available() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	default:
		return false
	}
}

var initialized = false

// Initialize will initialize the BLE api. The function must be called before
// using other functions of this package. BLE availability can be checked by
// calling Available() beforehand.
func Initialize() error {
	// return immediately if already initialized
	if initialized {
		return nil
	}

	// return immediately if not available
	if !Available() {
		return errors.New("BLE not available")
	}

	// initialize darwin system
	if runtime.GOOS == "darwin" {
		// create a device
		dev, err := darwin.NewDevice(darwin.OptCentralRole())
		if err != nil {
			return err
		}

		// set default device
		ble.SetDefaultDevice(dev)
	}

	// initialize linux system
	if runtime.GOOS == "linux" {
		// create a device
		dev, err := linux.NewDevice()
		if err != nil {
			return err
		}

		// set default device
		ble.SetDefaultDevice(dev)
	}

	// set flag
	initialized = true

	return nil
}
