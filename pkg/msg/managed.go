package msg

import (
	"fmt"
	"sync"
	"time"
)

// ManagedDevice represents a managed device.
type ManagedDevice struct {
	device   Device
	channel  Channel
	session  *Session
	password string
	mutex    sync.Mutex
}

// NewManagedDevice creates a new managed device.
func NewManagedDevice(dev Device) *ManagedDevice {
	// create active device
	ad := &ManagedDevice{
		device: dev,
	}

	// run pinger
	go ad.pinger()

	return ad
}

// Device returns the underlying device.
func (d *ManagedDevice) Device() Device {
	return d.device
}

// Activate activates the device.
func (d *ManagedDevice) Activate() error {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check channel
	if d.channel != nil {
		return nil
	}

	// open channel
	ch, err := d.device.Open()
	if err != nil {
		return err
	}

	// set channel
	d.channel = ch

	return nil
}

// Active returns whether the device is active.
func (d *ManagedDevice) Active() bool {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.channel != nil
}

// Locked returns whether the device is locked.
func (d *ManagedDevice) Locked() bool {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.session != nil
}

// NewSession creates a new session.
func (d *ManagedDevice) NewSession() (*Session, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check state
	if d.channel == nil {
		return nil, fmt.Errorf("device not active")
	}

	// open new session
	session, err := OpenSession(d.channel)
	if err != nil {
		return nil, err
	}

	// get status
	status, err := session.Status(time.Second)
	if err != nil {
		return nil, err
	}

	// try to unlock if password is available and locked
	if d.password != "" && (status&StatusLocked != 0) {
		_, err := session.Unlock(d.password, time.Second)
		if err != nil {
			return nil, err
		}
	}

	return session, nil
}

// UseSession uses the managed session.
func (d *ManagedDevice) UseSession(fn func(*Session) error) error {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check state
	if d.channel == nil {
		return fmt.Errorf("device not active")
	}

	// create session if missing
	if d.session == nil {
		// open new session
		session, err := OpenSession(d.channel)
		if err != nil {
			return err
		}

		// get status
		status, err := session.Status(time.Second)
		if err != nil {
			return err
		}

		// try to unlock if password is available and locked
		if d.password != "" && status&StatusLocked != 0 {
			_, err := session.Unlock(d.password, time.Second)
			if err != nil {
				return err
			}
		}

		// set session
		d.session = session
	}

	// run function
	err := fn(d.session)
	if err == nil {
		return nil
	}

	// close session
	_ = d.session.End(time.Second)
	d.session = nil

	return err
}

// Deactivate deactivates the device.
func (d *ManagedDevice) Deactivate() {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check channel
	if d.channel == nil {
		return
	}

	// end session if available
	if d.session != nil {
		_ = d.session.End(time.Second)
		d.session = nil
	}

	// close channel
	d.channel.Close()
	d.channel = nil
}

// Stop stops device management.
func (d *ManagedDevice) Stop() {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// close session
	if d.session != nil {
		_ = d.session.End(time.Second)
		d.session = nil
	}

	// close channel
	if d.channel != nil {
		d.channel.Close()
		d.channel = nil
	}

	// clear state
	d.device = nil
	d.password = ""
}

func (d *ManagedDevice) pinger() {
	for {
		// sleep
		time.Sleep(time.Second)

		// acquire mutex
		d.mutex.Lock()

		// check state
		if d.device == nil {
			d.mutex.Unlock()
			return
		}

		// ping session if available
		if d.session != nil {
			_ = d.session.Ping(time.Second)
		}

		// release mutex
		d.mutex.Unlock()
	}
}
