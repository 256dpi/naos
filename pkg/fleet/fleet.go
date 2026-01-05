// Package fleet provides a low-level implementation of the NAOS fleet management
// protocol.
package fleet

import (
	"encoding/json"
	"os"
	"time"

	"github.com/ryanuber/go-glob"
)

// A Device represents a single device in a fleet.
type Device struct {
	BaseTopic  string            `json:"base_topic"`
	DeviceName string            `json:"device_name"`
	AppName    string            `json:"app_name"`
	AppVersion string            `json:"app_version"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Metrics    []string          `json:"metrics,omitempty"`
}

// A Fleet represents a fleet of devices.
type Fleet struct {
	Broker  string             `json:"broker,omitempty"`
	Devices map[string]*Device `json:"devices,omitempty"`
}

// NewFleet creates a new fleet.
func NewFleet() *Fleet {
	return &Fleet{
		Broker: "tcp://localhost:1883",
	}
}

// ReadFleet will attempt to read the fleet at the specified path.
func ReadFleet(path string) (*Fleet, error) {
	// read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// decode data
	var f Fleet
	err = json.Unmarshal(data, &f)
	if err != nil {
		return nil, err
	}

	// create map of devices if missing
	if f.Devices == nil {
		f.Devices = make(map[string]*Device)
	}

	// iterate over all devices and initialize parameters
	for _, device := range f.Devices {
		if device.Parameters == nil {
			device.Parameters = make(map[string]string)
		}
	}

	return &f, nil
}

// Save will write the fleet to the specified path.
func (f *Fleet) Save(path string) error {
	// encode data
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	// write file
	err = os.WriteFile(path, append(data, '\n'), 0644)
	if err != nil {
		return err
	}

	return nil
}

// FilterDevices will return a list of devices that have a name matching the
// supplied glob pattern.
func (f *Fleet) FilterDevices(pattern string) []*Device {
	// collect matching devices
	var devices []*Device
	for name, device := range f.Devices {
		if glob.Glob(pattern, name) {
			devices = append(devices, device)
		}
	}

	return devices
}

// FindDevice returns the first device that has the matching base topic.
func (f *Fleet) FindDevice(baseTopic string) *Device {
	// find matching device
	for _, d := range f.Devices {
		if d.BaseTopic == baseTopic {
			return d
		}
	}

	return nil
}

// Collect will collect and update the flet with found devices for the given
// duration. It will return a list of devices that have been added to the fleet.
func (f *Fleet) Collect(duration time.Duration) ([]*Device, error) {
	// collect announcements
	ann, err := Collect(f.Broker, duration)
	if err != nil {
		return nil, err
	}

	// ensure devices
	if f.Devices == nil {
		f.Devices = make(map[string]*Device)
	}

	// prepare list
	var collected []*Device

	// handle all announcements
	for _, a := range ann {
		// get current device or add one if not existing
		d, ok := f.Devices[a.DeviceName]
		if !ok {
			d = &Device{
				DeviceName: a.DeviceName,
				Parameters: make(map[string]string),
			}
			f.Devices[a.DeviceName] = d
			collected = append(collected, d)
		}

		// update fields
		d.BaseTopic = a.BaseTopic
		d.AppName = a.AppName
		d.AppVersion = a.AppVersion
	}

	return collected, nil
}

// Discover will request all parameters and metrics from all matching devices.
// The fleet is updated with the reported parameters and metrics, and a list of
// queried devices is returned.
func (f *Fleet) Discover(pattern string, jobs int) ([]*Device, error) {
	// discover parameters and metrics
	results, err := Discover(f.Broker, BaseTopics(f.FilterDevices(pattern)), jobs)
	if err != nil {
		return nil, err
	}

	// prepare list
	var queried []*Device

	// update devices
	for baseTopic, result := range results {
		device := f.FindDevice(baseTopic)
		if device != nil {
			device.Parameters = result.Params
			device.Metrics = result.Metrics
			queried = append(queried, device)
		}
	}

	return queried, nil
}

// Ping will send a ping message to all matching devices.
func (f *Fleet) Ping(pattern string, jobs int) error {
	_, err := f.SetParams(pattern, "ping", "", jobs)
	return err
}

// GetParams will request specified parameter from all matching devices. The
// fleet is updated with the reported value and a list of queried devices is
// returned.
func (f *Fleet) GetParams(pattern, param string, jobs int) ([]*Device, error) {
	// set parameter
	table, err := GetParams(f.Broker, param, BaseTopics(f.FilterDevices(pattern)), jobs)
	if err != nil {
		return nil, err
	}

	// prepare list
	var queried []*Device

	// update device
	for baseTopic, value := range table {
		device := f.FindDevice(baseTopic)
		if device != nil {
			device.Parameters[param] = value
			queried = append(queried, device)
		}
	}

	return queried, nil
}

// SetParams will set the specified parameter on all matching devices. The fleet
// is updated with the saved value and a list of updated devices is returned.
func (f *Fleet) SetParams(pattern, param, value string, jobs int) ([]*Device, error) {
	// set parameter
	table, err := SetParams(f.Broker, param, value, BaseTopics(f.FilterDevices(pattern)), jobs)
	if err != nil {
		return nil, err
	}

	// prepare list of updated devices
	var updated []*Device

	// update device
	for baseTopic, value := range table {
		device := f.FindDevice(baseTopic)
		if device != nil {
			device.Parameters[param] = value
			updated = append(updated, device)
		}
	}

	return updated, nil
}

// UnsetParams will unset the specified parameter on all matching devices. The
// fleet is updated with the unset value and a list of updated devices is
// returned.
func (f *Fleet) UnsetParams(pattern, param string, jobs int) ([]*Device, error) {
	// set parameter
	err := UnsetParams(f.Broker, param, BaseTopics(f.FilterDevices(pattern)), jobs)
	if err != nil {
		return nil, err
	}

	// prepare list
	var updated []*Device

	// update device
	for _, device := range f.FilterDevices(pattern) {
		delete(device.Parameters, param)
		updated = append(updated, device)
	}

	return updated, nil
}

// Record will enable log recording on all matching devices and yield the
// received log messages until the provided channel has been closed.
func (f *Fleet) Record(pattern string, quit chan struct{}, callback func(time.Time, *Device, string)) error {
	return Record(f.Broker, BaseTopics(f.FilterDevices(pattern)), quit, func(log *LogMessage) {
		// call user callback
		if callback != nil {
			callback(log.Time, f.FindDevice(log.BaseTopic), log.Content)
		}
	})
}

// Monitor will monitor the matching devices and update the fleet accordingly.
// The specified callback is called for every heartbeat with the updated device
// and the received heartbeat.
func (f *Fleet) Monitor(pattern string, quit chan struct{}, callback func(*Device, *Heartbeat)) error {
	return Monitor(f.Broker, BaseTopics(f.FilterDevices(pattern)), quit, func(heartbeat *Heartbeat) {
		// get device
		device, ok := f.Devices[heartbeat.DeviceName]
		if !ok {
			return
		}

		// update fields
		device.DeviceName = heartbeat.DeviceName
		device.AppName = heartbeat.AppName
		device.AppVersion = heartbeat.AppVersion

		// call user callback
		if callback != nil {
			callback(device, heartbeat)
		}
	})
}

// Debug will load the latest coredump from all matching devices. If delete is
// true, the coredumps are deleted from the devices after retrieval. A table of
// devices and their corresponding coredumps is returned.
func (f *Fleet) Debug(pattern string, delete bool, jobs int) (map[*Device][]byte, error) {
	// gather coredumps
	coredumps, err := Debug(f.Broker, BaseTopics(f.FilterDevices(pattern)), delete, jobs)
	if err != nil {
		return nil, err
	}

	// create new table
	table := make(map[*Device][]byte, len(coredumps))

	// fill table
	for baseTopic, coredump := range coredumps {
		// ignore zero length coredump
		if len(coredump) == 0 {
			continue
		}

		// add entry
		table[f.FindDevice(baseTopic)] = coredump
	}

	return table, nil
}

// Update will update all matching devices with the specified image. The
// specified callback is called for every change in state or progress.
func (f *Fleet) Update(version, pattern string, firmware []byte, jobs int, callback func(*Device, UpdateStatus)) error {
	// get devices
	list := f.FilterDevices(pattern)

	// filter by version
	var devices []*Device
	for _, d := range list {
		if d.AppVersion != version {
			devices = append(devices, d)
		}
	}

	// update devices
	_, err := Update(f.Broker, BaseTopics(devices), firmware, jobs, func(baseTopic string, status UpdateStatus) {
		// get device
		device := f.FindDevice(baseTopic)
		if device == nil {
			return
		}

		// call callback
		callback(device, status)
	})
	if err != nil {
		return err
	}

	return nil
}

// BaseTopics returns a list of base topics from the provided devices.
func BaseTopics(devices []*Device) []string {
	// prepare list
	var l []string

	// add all matching devices
	for _, d := range devices {
		l = append(l, d.BaseTopic)
	}

	return l
}
