package main

import (
	"sync"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

type managedDevice struct {
	dev msg.Device
	ch  msg.Channel
	ps  *msg.Session
	mu  sync.Mutex

	Params map[string]msg.ParamInfo
	Values map[uint8][]byte
}

func newManagedDevice(dev msg.Device) *managedDevice {
	// create active device
	ad := &managedDevice{
		dev: dev,
	}

	// run task
	go ad.run()

	return ad
}

func (d *managedDevice) Device() msg.Device {
	return d.dev
}

func (d *managedDevice) Get(name string) []byte {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

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

func (d *managedDevice) Active() bool {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.ch != nil
}

func (d *managedDevice) Activate() error {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// check channel
	if d.ch != nil {
		return nil
	}

	// open channel
	ch, err := d.dev.Open()
	if err != nil {
		return err
	}

	// set channel
	d.ch = ch

	return nil
}

// Refresh refreshes the device.
func (d *managedDevice) Refresh() error {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// check channel
	if d.ch == nil {
		return nil
	}

	// open session
	ps, err := msg.OpenSession(d.ch)
	if err != nil {
		return err
	}

	// ensure end
	defer ps.End(time.Second)

	// list params
	params, err := msg.ListParams(ps, time.Second)
	if err != nil {
		return err
	}

	// set params
	d.Params = map[string]msg.ParamInfo{}
	for _, param := range params {
		d.Params[param.Name] = param
	}

	// read values
	d.Values = map[uint8][]byte{}
	for _, info := range d.Params {
		if info.Type == msg.ParamTypeAction {
			continue
		}
		value, err := msg.ReadParam(ps, info.Ref, time.Second)
		if err != nil {
			return err
		}
		d.Values[info.Ref] = value
	}

	return nil
}

func (d *managedDevice) Deactivate() {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// check channel
	if d.ch == nil {
		return
	}

	// close channel
	d.ch.Close()
	d.ch = nil
}

func (d *managedDevice) run() {
	for {
		// run check
		d.check()

		// sleep
		time.Sleep(time.Second)
	}
}

func (d *managedDevice) check() {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// return if inactive
	if d.ch == nil {
		return
	}
}
