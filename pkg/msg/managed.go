package msg

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrManagedStopped is returned when using a stopped managed device.
var ErrManagedStopped = errors.New("managed device stopped")

// ManagedEventType represents the type of a managed device event.
type ManagedEventType int

const (
	// ManagedConnected is emitted after a successful activate transition.
	ManagedConnected ManagedEventType = iota

	// ManagedDisconnected is emitted on unexpected transport loss.
	ManagedDisconnected
)

// ManagedEvent represents a lifecycle event from a managed device.
type ManagedEvent struct {
	Type ManagedEventType
}

// ManagedDevice represents a managed device.
type ManagedDevice struct {
	device   Device
	channel  *Channel
	session  *Session
	password string
	locked   bool
	subs     map[uint64]chan ManagedEvent
	nextSub  uint64
	stopped  bool
	mutex    sync.Mutex
}

// NewManagedDevice creates a new managed device.
func NewManagedDevice(dev Device) *ManagedDevice {
	// create active device
	ad := &ManagedDevice{
		device: dev,
		subs:   make(map[uint64]chan ManagedEvent),
	}

	// run pinger
	go ad.pinger()

	return ad
}

// Device returns the underlying device.
func (d *ManagedDevice) Device() Device {
	return d.device
}

// Events returns a new event channel and a cancel function. The channel is
// buffered (4) and drops events if the consumer is too slow. Call cancel to
// unsubscribe and close the channel.
func (d *ManagedDevice) Events() (<-chan ManagedEvent, func()) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// create channel
	ch := make(chan ManagedEvent, 4)

	// close immediately if stopped
	if d.stopped {
		close(ch)
		return ch, func() {}
	}

	// add subscriber
	id := d.nextSub
	d.nextSub++
	d.subs[id] = ch

	return ch, func() {
		d.mutex.Lock()
		defer d.mutex.Unlock()

		sub, ok := d.subs[id]
		if !ok {
			return
		}

		delete(d.subs, id)
		close(sub)
	}
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
	if d.stopped {
		return ErrManagedStopped
	}

	// open channel
	ch, err := d.device.Open()
	if err != nil {
		return err
	}

	// set channel
	d.channel = ch

	// read lock status
	d.session, err = d.openSession()
	if err != nil {
		d.channel.Close()
		d.channel = nil
		return err
	}

	// emit connected
	d.emit(ManagedEvent{Type: ManagedConnected})

	// watch for transport loss
	go d.watcher(ch)

	return nil
}

// Active returns whether the device is active.
func (d *ManagedDevice) Active() bool {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.channel != nil
}

// HasSession returns whether the device has a managed session.
func (d *ManagedDevice) HasSession() bool {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.session != nil
}

// Locked returns whether the device is locked. The value is determined during
// session opening and may not reflect the current state.
func (d *ManagedDevice) Locked() bool {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.locked
}

// Unlock will attempt to unlock the device and returns its success. The
// password is stored for auto-unlock on success.
func (d *ManagedDevice) Unlock(password string) (bool, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check state
	if d.stopped {
		return false, ErrManagedStopped
	}
	if d.channel == nil {
		return false, fmt.Errorf("device not active")
	}

	// ensure session
	if d.session == nil {
		session, err := d.openSession()
		if err != nil {
			return false, err
		}
		d.session = session
	}

	// unlock
	ok, err := d.session.Unlock(password, time.Second)
	if err != nil {
		_ = d.session.End(time.Second)
		d.session = nil
		return false, err
	}

	// store password and update locked state on success
	if ok {
		d.password = password
		d.locked = false
	}

	return ok, nil
}

// NewSession creates a new session.
func (d *ManagedDevice) NewSession() (*Session, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check state
	if d.stopped {
		return nil, ErrManagedStopped
	}
	if d.channel == nil {
		return nil, fmt.Errorf("device not active")
	}

	return d.openSession()
}

// UseSession uses the managed session.
func (d *ManagedDevice) UseSession(fn func(*Session) error) error {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check state
	if d.stopped {
		return ErrManagedStopped
	}
	if d.channel == nil {
		return fmt.Errorf("device not active")
	}

	// create session if missing
	if d.session == nil {
		session, err := d.openSession()
		if err != nil {
			return err
		}
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

func (d *ManagedDevice) openSession() (*Session, error) {
	// open new session
	session, err := OpenSession(d.channel, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// get status
	status, err := session.Status(time.Second)
	if err != nil {
		return nil, err
	}

	// update locked state
	d.locked = status&StatusLocked != 0

	// try to unlock if password is available and locked
	if d.password != "" && d.locked {
		ok, err := session.Unlock(d.password, time.Second)
		if err != nil {
			return nil, err
		}
		if ok {
			d.locked = false
		}
	}

	return session, nil
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

	// close all subscriber channels
	for _, ch := range d.subs {
		close(ch)
	}
	d.subs = nil
	d.stopped = true

	// clear state
	d.device = nil
	d.password = ""
}

func (d *ManagedDevice) emit(event ManagedEvent) {
	for _, ch := range d.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (d *ManagedDevice) watcher(ch *Channel) {
	// wait for channel close
	<-ch.Done()

	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// skip if channel was replaced (intentional deactivate)
	if d.channel != ch {
		return
	}

	// clear session
	if d.session != nil {
		_ = d.session.End(time.Second)
		d.session = nil
	}

	// clear channel
	d.channel = nil

	// emit disconnected
	d.emit(ManagedEvent{Type: ManagedDisconnected})
}

func (d *ManagedDevice) pinger() {
	for {
		// sleep
		time.Sleep(5 * time.Second)

		// acquire mutex
		d.mutex.Lock()

		// check state
		if d.device == nil {
			d.mutex.Unlock()
			return
		}

		// ping session if available
		if d.session != nil {
			if err := d.session.Ping(5 * time.Second); err != nil {
				_ = d.session.End(time.Second)
				d.session = nil
			}
		}

		// release mutex
		d.mutex.Unlock()
	}
}
