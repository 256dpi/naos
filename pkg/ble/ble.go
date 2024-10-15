package ble

import (
	"fmt"
	"io"

	"github.com/samber/lo"
	"tinygo.org/x/bluetooth"

	"github.com/256dpi/naos/pkg/utils"
)

var adapter = bluetooth.DefaultAdapter
var serviceUUID = lo.Must(bluetooth.ParseUUID("632FBA1B-4861-4E4F-8103-FFEE9D5033B5"))
var selectUUID = lo.Must(bluetooth.ParseUUID("CFC9706D-406F-CCBE-4240-F88D6ED4BACD"))
var valueUUID = lo.Must(bluetooth.ParseUUID("01CA5446-8EE1-7E99-2041-6884B01E71B3"))

// Config configures all reachable BLE device with the given parameters.
func Config(params map[string]string, out io.Writer) error {
	// enable BLE adapter
	err := adapter.Enable()
	if err != nil {
		return err
	}

	// prepare map
	devices := map[string]bool{}

	// log info
	utils.Log(out, "Scanning for devices... (press Ctrl+C to stop)")

	// start scanning
	err = adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		// check service
		if !device.HasServiceUUID(serviceUUID) {
			return
		}

		// check map
		if devices[device.Address.String()] {
			return
		}

		// mark device
		devices[device.Address.String()] = true

		go func(localName string) {
			// connect to device
			device, err := adapter.Connect(device.Address, bluetooth.ConnectionParams{})
			if err != nil {
				utils.Log(out, fmt.Sprintf("Error: %s", err))
				return
			}

			// ensure disconnect when done
			defer func() {
				err := device.Disconnect()
				if err != nil {
					utils.Log(out, fmt.Sprintf("Error: %s", err))
				}
			}()

			// discover services
			svcs, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
			if err != nil {
				utils.Log(out, fmt.Sprintf("Error: %s", err))
				return
			}

			// check services
			if len(svcs) != 1 {
				utils.Log(out, fmt.Sprintf("Error: unexpected number of services: %d", len(svcs)))
				return
			}

			// discover characteristics
			chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{
				selectUUID,
				valueUUID,
			})
			if err != nil {
				utils.Log(out, fmt.Sprintf("Error: %s", err))
				return
			}

			// check characteristics
			if len(chars) != 2 {
				utils.Log(out, fmt.Sprintf("Error: unexpected number of characteristics: %d", len(chars)))
				return
			}

			// find characteristics
			var selectChar, valueChar bluetooth.DeviceCharacteristic
			for _, char := range chars {
				switch char.UUID() {
				case selectUUID:
					selectChar = char
				case valueUUID:
					valueChar = char
				default:
					utils.Log(out, fmt.Sprintf("Error: unexpected characteristic: %s", char.UUID()))
					return
				}
			}

			// write parameters
			for param, value := range params {
				// select parameter
				_, err = selectChar.Write([]byte(param))
				if err != nil {
					utils.Log(out, fmt.Sprintf("Error: %s", err))
					return
				}

				// write value
				_, err = valueChar.Write([]byte(value))
				if err != nil {
					utils.Log(out, fmt.Sprintf("Error: %s", err))
					return
				}
			}

			// log success
			utils.Log(out, fmt.Sprintf("Configured: %s", localName))
		}(device.LocalName())
	})

	return nil
}
