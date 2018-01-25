package ble

import (
	"context"
	"time"

	"github.com/go-ble/ble"
)

var serviceUUID = ble.MustParse("632FBA1B-4861-4E4F-8103-FFEE9D5033B5")

// Scan will scan for available NAOS devices and return their addresses.
func Scan(duration time.Duration) ([]string, error) {
	// create cancelable context with a duration
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// scan for devices
	advertisements, err := ble.Find(ctx, false, nil)
	if err != nil && err != context.DeadlineExceeded {
		return nil, err
	}

	// prepare list of addresses
	var addresses []string

	// go through all advertisements
	for _, a := range advertisements {
		// skip devices not named "naos"
		if a.LocalName() != "naos" {
			continue
		}

		// skip devices that do not advertise the primary service
		services := a.Services()
		if len(services) != 1 || !services[0].Equal(serviceUUID) {
			continue
		}

		// add device address
		addresses = append(addresses, a.Addr().String())
	}

	return addresses, nil
}
