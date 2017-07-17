package naos

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/darwin"
)

var serviceUUID = ble.MustParse("632FBA1B-4861-4E4F-8103-FFEE9D5033B5")
var wifiSSIDUUID = ble.MustParse("802DD327-CA04-4C90-BE86-A3568275A510")
var wifiPasswordUUID = ble.MustParse("B3883261-F360-4CB7-9791-C3498FB2C151")
var mqttHostUUID = ble.MustParse("193FFFF2-4542-4EBC-BE1F-4A355D40AC57")
var mqttPortUUID = ble.MustParse("CB8A764C-546C-4787-9A6F-427382D49755")
var mqttClientIDUUID = ble.MustParse("08C4E543-65CC-4BF6-BBA8-C8A5BF916C3B")
var mqttUsernameUUID = ble.MustParse("ABB4A59A-85E9-449C-80F5-72D806A00257")
var mqttPasswordUUID = ble.MustParse("C5ECB1B1-0658-4FC3-8052-215521D41925")
var deviceTypeUUID = ble.MustParse("0CEA3120-196E-4308-8883-8EA8757B9191")
var deviceNameUUID = ble.MustParse("25427850-28F3-40EA-926A-AB3CEB6D0B56")
var baseTopicUUID = ble.MustParse("EAB7E3A8-9FF8-4938-8EA2-D09B8C63CAB4")
var connectionStatusUUID = ble.MustParse("59997CE0-3A50-433C-A200-D9F6D312EB7C")
var systemCommandUUID = ble.MustParse("37CF1864-5A8E-450F-A277-2981BD76D0AB")

// BLEDevice is returned by Scan.
type BLEDevice struct {
	WiFiSSID         string
	WiFiPassword     string
	MQTTHost         string
	MQTTPort         int
	MQTTClientID     string
	MQTTUsername     string
	MQTTPassword     string
	DeviceType       string
	DeviceName       string
	BaseTopic        string
	ConnectionStatus string
}

// Scan will scan for available Bluetooth devices.
func Scan(duration, timeout time.Duration) ([]*BLEDevice, error) {
	// create a device TODO: Support linux.
	dev, err := darwin.NewDevice(darwin.OptCentralRole())
	if err != nil {
		return nil, err
	}

	// set default device
	ble.SetDefaultDevice(dev)

	// scan for addresses
	addresses, err := bleScan(duration)
	if err != nil {
		return nil, err
	}

	// prepare list
	var list []*BLEDevice

	// explore devices
	for _, a := range addresses {
		device, err := bleExplore(a, timeout)
		if err != nil {
			return nil, err
		}

		list = append(list, device)
	}

	return list, nil
}

func bleScan(duration time.Duration) ([]ble.Addr, error) {
	// create cancelable context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), duration)

	// make sure cancel will be called
	defer cancel()

	// scan for devices
	advertisements, err := ble.Find(ctx, false, nil)
	if err != nil && err != context.DeadlineExceeded {
		return nil, err
	}

	// prepare list of addresses
	var addresses []ble.Addr

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
		addresses = append(addresses, a.Address())
	}

	return addresses, nil
}

func bleExplore(addr ble.Addr, timeout time.Duration) (*BLEDevice, error) {
	// create a new cancelable context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// make sure cancel will be called
	defer cancel()

	// connect to device
	client, err := ble.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}

	// discover services
	services, err := client.DiscoverServices([]ble.UUID{serviceUUID})
	if err != nil {
		return nil, err
	}

	// check service count
	if len(services) != 1 {
		return nil, errors.New("primary service not found")
	}

	// discover characteristics
	characteristics, err := client.DiscoverCharacteristics(nil, services[0])
	if err != nil {
		return nil, err
	}

	// prepare device
	device := &BLEDevice{}

	// go through all characteristics
	for _, c := range characteristics {
		if c.UUID.Equal(wifiSSIDUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.WiFiSSID = string(data)
		} else if c.UUID.Equal(wifiPasswordUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.WiFiPassword = string(data)
		} else if c.UUID.Equal(mqttHostUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.MQTTHost = string(data)
		} else if c.UUID.Equal(mqttPortUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.MQTTPort, _ = strconv.Atoi(string(data))
		} else if c.UUID.Equal(mqttClientIDUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.MQTTClientID = string(data)
		} else if c.UUID.Equal(mqttUsernameUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.MQTTUsername = string(data)
		} else if c.UUID.Equal(mqttPasswordUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.MQTTPassword = string(data)
		} else if c.UUID.Equal(deviceTypeUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.DeviceType = string(data)
		} else if c.UUID.Equal(deviceNameUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.DeviceName = string(data)
		} else if c.UUID.Equal(baseTopicUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.BaseTopic = string(data)
		} else if c.UUID.Equal(connectionStatusUUID) {
			data, err := client.ReadCharacteristic(c)
			if err != nil {
				return nil, err
			}

			device.ConnectionStatus = string(data)
		}
	}

	return device, nil
}
