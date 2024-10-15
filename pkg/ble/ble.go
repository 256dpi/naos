package ble

import (
	"fmt"
	"io"

	"github.com/samber/lo"
	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter
var serviceUUID = lo.Must(bluetooth.ParseUUID("632FBA1B-4861-4E4F-8103-FFEE9D5033B5"))
var selectUUID = lo.Must(bluetooth.ParseUUID("CFC9706D-406F-CCBE-4240-F88D6ED4BACD"))
var valueUUID = lo.Must(bluetooth.ParseUUID("01CA5446-8EE1-7E99-2041-6884B01E71B3"))

func Assign(param, value string, out io.Writer) error {
	// enable BLE adapter
	err := adapter.Enable()
	if err != nil {
		return err
	}

	// prepare map
	devices := map[string]bool{}

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
				fmt.Fprintf(out, "error: %s\n", err)
				return
			}

			// ensure disconnect when done
			defer func() {
				err := device.Disconnect()
				if err != nil {
					fmt.Fprintf(out, "error: %s\n", err)
				}
			}()

			// discover services
			svcs, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
			if err != nil {
				fmt.Fprintf(out, "error: %s\n", err)
				return
			}

			// check services
			if len(svcs) != 1 {
				fmt.Fprintf(out, "error: unexpected number of services: %d\n", len(svcs))
				return
			}

			// discover characteristics
			chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{
				selectUUID,
				valueUUID,
			})
			if err != nil {
				fmt.Fprintf(out, "error: %s\n", err)
				return
			}

			// check characteristics
			if len(chars) != 2 {
				fmt.Fprintf(out, "error: unexpected number of characteristics: %d\n", len(chars))
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
					fmt.Fprintf(out, "error: unexpected characteristic: %s\n", char.UUID())
					return
				}
			}

			// select parameter
			_, err = selectChar.Write([]byte(param))
			if err != nil {
				fmt.Fprintf(out, "error: %s\n", err)
				return
			}

			// write value
			_, err = valueChar.Write([]byte(value))
			if err != nil {
				fmt.Fprintf(out, "error: %s\n", err)
				return
			}

			// log success
			fmt.Fprintf(out, "==> Assigned: %s\n", localName)
		}(device.LocalName())
	})

	// scanning blocks, use ctrl+c to stop for now

	return nil
}
