package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

type device struct {
	md  *msg.ManagedDevice
	age time.Time
	ps  *msg.ParamsService
	ms  *msg.MetricsService
	pss *msg.Session
	mss *msg.Session
	mut sync.Mutex

	deviceName string
	appName    string
	appVersion string
}

func newDevice(dev msg.Device, deviceName, appName, appVersion string) *device {
	return &device{
		md:         msg.NewManagedDevice(dev),
		age:        time.Now(),
		deviceName: deviceName,
		appName:    appName,
		appVersion: appVersion,
	}
}

func (d *device) DeviceName() string {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.deviceName
}

func (d *device) AppName() string {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.appName
}

func (d *device) AppVersion() string {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.appVersion
}

func (d *device) ID() string {
	return d.md.Device().ID()
}

func (d *device) LastSeen() time.Time {
	// acquire mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	return d.age
}

func (d *device) UpdateLastSeen(ts time.Time) {
	// acquire mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	d.age = ts
}

func (d *device) Active() bool {
	return d.md.Active()
}

func (d *device) ParamsService() *msg.ParamsService {
	// acquire mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	return d.ps
}

func (d *device) MetricsService() *msg.MetricsService {
	// acquire mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	return d.ms
}

func (d *device) Activate() error {
	// lock mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	// check active
	if d.md.Active() {
		return fmt.Errorf("device already active")
	}

	// activate device
	err := d.md.Activate()
	if err != nil {
		return fmt.Errorf("failed to activate device: %w", err)
	}

	// create params service
	d.pss, err = d.md.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create param session: %w", err)
	}

	// set up params service
	d.ps = msg.NewParamsService(d.pss)

	// list params
	err = d.ps.List()
	if err != nil {
		return fmt.Errorf("failed to list params: %w", err)
	}

	// create metrics service
	d.mss, err = d.md.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create metric session: %w", err)
	}

	// set up metrics service
	d.ms = msg.NewMetricsService(d.mss)

	// list metrics
	err = d.ms.List()
	if err != nil {
		return fmt.Errorf("failed to list metrics: %w", err)
	}

	return nil
}

func (d *device) Deactivate() {
	// acquire mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	// end param session
	if d.pss != nil {
		_ = d.pss.End(5 * time.Second)
		d.pss = nil
	}
	d.ps = nil

	// end metric session
	if d.mss != nil {
		_ = d.mss.End(5 * time.Second)
		d.mss = nil
	}
	d.ms = nil

	// deactivate device
	d.md.Deactivate()
}

func (d *device) WithSession(fn func(*msg.Session) error) error {
	// acquire mutex
	d.mut.Lock()
	defer d.mut.Unlock()

	// use session
	err := d.md.UseSession(fn)
	if err != nil {
		return fmt.Errorf("failed to use session: %w", err)
	}

	return nil
}
