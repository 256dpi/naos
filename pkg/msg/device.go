package msg

import (
	"sync"
	"time"
)

// Device represents a device that can be communicated with.
type Device interface {
	Addr() string
	Channel() (Channel, error)
}

// ManagedDevice represents a device that is managed.
type ManagedDevice struct {
	dev Device
	ch  Channel
	ps  *Session
	mu  sync.Mutex

	Params map[string]ParamInfo
	Values map[uint8][]byte
}

// NewManagedDevice creates a new managed device.
func NewManagedDevice(dev Device) *ManagedDevice {
	// create active device
	ad := &ManagedDevice{
		dev: dev,
	}

	// run task
	go ad.run()

	return ad
}

// Addr returns the address of the device.
func (d *ManagedDevice) Addr() string {
	return d.dev.Addr()
}

// Get returns the value of a parameter.
func (d *ManagedDevice) Get(name string) []byte {
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

// GetString returns the string value of a parameter.
func (d *ManagedDevice) GetString(name string) string {
	// get value
	value := d.Get(name)
	if value == nil {
		return ""
	}

	return string(value)
}

// Active returns whether the device is active.
func (d *ManagedDevice) Active() bool {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.ch != nil
}

// Activate activates the device.
func (d *ManagedDevice) Activate() error {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// check channel
	if d.ch != nil {
		return nil
	}

	// open channel
	ch, err := d.dev.Channel()
	if err != nil {
		return err
	}

	// set channel
	d.ch = ch

	return nil
}

// Refresh refreshes the device.
func (d *ManagedDevice) Refresh() error {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// check channel
	if d.ch == nil {
		return nil
	}

	// open session
	ps, err := OpenSession(d.ch)
	if err != nil {
		return err
	}

	// ensure end
	defer ps.End(time.Second)

	// list params
	params, err := ListParams(ps, time.Second)
	if err != nil {
		return err
	}

	// set params
	d.Params = map[string]ParamInfo{}
	for _, param := range params {
		d.Params[param.Name] = param
	}

	// read values
	d.Values = map[uint8][]byte{}
	for _, info := range d.Params {
		if info.Type == ParamTypeAction {
			continue
		}
		value, err := ReadParam(ps, info.Ref, time.Second)
		if err != nil {
			return err
		}
		d.Values[info.Ref] = value
	}

	return nil
}

// Deactivate deactivates the device.
func (d *ManagedDevice) Deactivate() {
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

func (d *ManagedDevice) run() {
	for {
		// run check
		d.check()

		// sleep
		time.Sleep(time.Second)
	}
}

func (d *ManagedDevice) check() {
	// acquire mutex
	d.mu.Lock()
	defer d.mu.Unlock()

	// return if inactive
	if d.ch == nil {
		return
	}
}
