package main

import (
	"sync"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

type managedDevice struct {
	Device *msg.ManagedDevice
	Params map[string]msg.ParamInfo
	Values map[uint8][]byte
	mutex  sync.Mutex
}

func newManagedDevice(dev msg.Device) *managedDevice {
	return &managedDevice{
		Device: msg.NewManagedDevice(dev),
	}
}

func (d *managedDevice) Get(name string) []byte {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// get param
	param, ok := d.Params[name]
	if !ok {
		return nil
	}

	// get value
	value, ok := d.Values[param.Ref]
	if !ok {
		return nil
	}

	return value
}

func (d *managedDevice) GetString(name string) string {
	// get value
	value := d.Get(name)
	if value == nil {
		return ""
	}

	return string(value)
}

// Refresh refreshes the device.
func (d *managedDevice) Refresh() error {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// list params
	var params []msg.ParamInfo
	err := d.Device.UseSession(func(s *msg.Session) error {
		var err error
		params, err = msg.ListParams(s, time.Second)
		return err
	})
	if err != nil {
		return err
	}

	// set params
	d.Params = map[string]msg.ParamInfo{}
	for _, param := range params {
		d.Params[param.Name] = param
	}

	// collect params
	var updates []msg.ParamUpdate
	err = d.Device.UseSession(func(s *msg.Session) error {
		var err error
		updates, err = msg.CollectParams(s, nil, 0, time.Second)
		return err
	})
	if err != nil {
		return err
	}

	// set values
	d.Values = map[uint8][]byte{}
	for _, update := range updates {
		d.Values[update.Ref] = update.Value
	}

	return nil
}
