package ble

import (
	"context"
	"errors"
	"sync"

	"github.com/currantlabs/ble"
)

// A Device is a BLE connected NAOS device.
type Device struct {
	client ble.Client
	mutex  sync.Mutex
}

// Connect will connect to the device with the specified address.
func Connect(addr string) (*Device, error) {
	// connect to device
	client, err := ble.Dial(context.Background(), ble.NewAddr(addr))
	if err != nil {
		return nil, err
	}

	// discover services
	_, err = client.DiscoverProfile(false)
	if err != nil {
		return nil, err
	}

	// check service count
	if len(client.Profile().Services) != 1 {
		return nil, errors.New("primary service not found")
	}

	// create device
	device := &Device{
		client: client,
	}

	return device, nil
}

// Address will return the device address.
func (d *Device) Address() string {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.client.Address().String()
}

// Read will read the specified property.
func (d *Device) Read(p Property) (string, error) {
	// check readability
	if !p.Readable() {
		return "", errors.New("property not readable")
	}

	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// get uuid
	uuid, err := p.uuid()
	if err != nil {
		return "", err
	}

	// find characteristic
	c := d.client.Profile().Find(ble.NewCharacteristic(uuid))
	if c == nil {
		return "", errors.New("characteristic not found")
	}

	// read characteristic
	data, err := d.client.ReadCharacteristic(c.(*ble.Characteristic))
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Write will write the specified property.
func (d *Device) Write(p Property, value string) error {
	// check readability
	if !p.Writable() {
		return errors.New("property not writable")
	}

	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// get uuid
	uuid, err := p.uuid()
	if err != nil {
		return err
	}

	// find characteristic
	c := d.client.Profile().Find(ble.NewCharacteristic(uuid))
	if c == nil {
		return errors.New("characteristic not found")
	}

	// write characteristic
	err = d.client.WriteCharacteristic(c.(*ble.Characteristic), []byte(value), true)
	if err != nil {
		return err
	}

	return nil
}

// Restart will command the device to restart some of its sub systems.
func (d *Device) Restart(wifi, mqtt bool) error {
	// restart wifi if requested
	if wifi {
		err := d.Write(systemCommandProperty, "restart-wifi")
		if err != nil {
			return err
		}
	}

	// restart mqtt if requested
	if mqtt {
		err := d.Write(systemCommandProperty, "restart-mqtt")
		if err != nil {
			return err
		}
	}

	return nil
}

// Close will close the connection to the device.
func (d *Device) Close() error {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// otherwise try to disconnect
	err := d.client.CancelConnection()
	if err != nil {
		return err
	}

	return nil
}
