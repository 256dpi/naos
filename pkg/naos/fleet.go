package naos

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/ryanuber/go-glob"

	"github.com/256dpi/naos/pkg/fleet"
)

// A Device represents a single device in a Fleet.
type Device struct {
	BaseTopic       string            `json:"base_topic"`
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	FirmwareVersion string            `json:"firmware_version"`
	Parameters      map[string]string `json:"parameters"`
}

// A Fleet represents the contents of the fleet file.
type Fleet struct {
	Broker  string             `json:"broker,omitempty"`
	Devices map[string]*Device `json:"devices,omitempty"`
}

// NewFleet creates a new Fleet.
func NewFleet() *Fleet {
	return &Fleet{}
}

// ReadFleet will attempt to read the fleet file at the specified path.
func ReadFleet(path string) (*Fleet, error) {
	// read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// decode data
	var flt Fleet
	err = json.Unmarshal(data, &flt)
	if err != nil {
		return nil, err
	}

	// create map of devices if missing
	if flt.Devices == nil {
		flt.Devices = make(map[string]*Device)
	}

	// iterate over all devices and initialize parameters
	for _, device := range flt.Devices {
		if device.Parameters == nil {
			device.Parameters = make(map[string]string)
		}
	}

	return &flt, nil
}

// Save will write the inventory file to the specified path.
func (f *Fleet) Save(path string) error {
	// encode data
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	// write config
	err = os.WriteFile(path, append(data, '\n'), 0644)
	if err != nil {
		return err
	}

	return nil
}

// FilterDevices will return a list of devices that have a name matching the supplied
// glob pattern.
func (f *Fleet) FilterDevices(pattern string) []*Device {
	// prepare list
	var devices []*Device

	// go over all devices
	for name, device := range f.Devices {
		// add name if it matches glob
		if glob.Glob(pattern, name) {
			devices = append(devices, device)
		}
	}

	return devices
}

// DeviceByBaseTopic returns the first device that has the matching base topic.
func (f *Fleet) DeviceByBaseTopic(baseTopic string) *Device {
	// iterate through all devices
	for _, d := range f.Devices {
		if d.BaseTopic == baseTopic {
			return d
		}
	}

	return nil
}

// Collect will collect announcements and update the inventory with found devices
// for the given amount of time. It will return a list of devices that have been
// added to the inventory.
func (f *Fleet) Collect(duration time.Duration) ([]*Device, error) {
	// collect announcements
	anns, err := fleet.Collect(f.Broker, duration)
	if err != nil {
		return nil, err
	}

	// ensure devices
	if f.Devices == nil {
		f.Devices = make(map[string]*Device)
	}

	// prepare list
	var newDevices []*Device

	// handle all announcements
	for _, a := range anns {
		// get current device or add one if not existing
		d, ok := f.Devices[a.DeviceName]
		if !ok {
			d = &Device{Name: a.DeviceName, Parameters: make(map[string]string)}
			f.Devices[a.DeviceName] = d
			newDevices = append(newDevices, d)
		}

		// update fields
		d.BaseTopic = a.BaseTopic
		d.Type = a.DeviceType
		d.FirmwareVersion = a.FirmwareVersion
	}

	return newDevices, nil
}

// Ping will send a ping message to all devices matching the supplied glob pattern.
func (f *Fleet) Ping(pattern string, timeout time.Duration) error {
	// get base topics
	baseTopics := BaseTopics(f.FilterDevices(pattern))

	// prepare new list
	topics := make([]string, 0, len(baseTopics))

	// generate topics
	for _, bt := range baseTopics {
		topics = append(topics, bt+"/naos/ping")
	}

	// send message to the generated topics
	err := fleet.Send(f.Broker, topics, "", timeout)
	if err != nil {
		return err
	}

	return nil
}

// Send will send a message to all devices matching the supplied glob pattern.
func (f *Fleet) Send(pattern, topic, message string, timeout time.Duration) error {
	// get base topics
	baseTopics := BaseTopics(f.FilterDevices(pattern))

	// prepare new list
	topics := make([]string, 0, len(baseTopics))

	// generate topics
	for _, bt := range baseTopics {
		topics = append(topics, bt+"/"+strings.Trim(topic, "/"))
	}

	// send message to the generated topics
	err := fleet.Send(f.Broker, topics, message, timeout)
	if err != nil {
		return err
	}

	return nil
}

// Receive will receive messages from all devices matching the supplied glob pattern.
func (f *Fleet) Receive(pattern, topic string, timeout time.Duration) (map[string]string, error) {
	// get base topics
	baseTopics := BaseTopics(f.FilterDevices(pattern))

	// prepare new list
	topics := make([]string, 0, len(baseTopics))

	// generate topics
	for _, bt := range baseTopics {
		topics = append(topics, bt+"/"+strings.Trim(topic, "/"))
	}

	// receive from the generated topics
	msgs, err := fleet.Receive(f.Broker, topics, timeout)
	if err != nil {
		return nil, err
	}

	return msgs, nil
}

// Discover will request the list of parameters from all devices matching the
// supplied glob pattern. The inventory is updated with the reported parameters
// and a list of answering devices is returned.
func (f *Fleet) Discover(pattern string, timeout time.Duration) ([]*Device, error) {
	// discover parameters
	table, err := fleet.Discover(f.Broker, BaseTopics(f.FilterDevices(pattern)), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of answering devices
	var answering []*Device

	// update device
	for baseTopic, parameters := range table {
		device := f.DeviceByBaseTopic(baseTopic)
		if device != nil {
			// initialize unset parameters
			for _, p := range parameters {
				if _, ok := device.Parameters[p]; !ok {
					device.Parameters[p] = ""
				}
			}

			answering = append(answering, device)
		}
	}

	return answering, nil
}

// GetParams will request specified parameter from all devices matching the supplied
// glob pattern. The inventory is updated with the reported value and a list of
// answering devices is returned.
func (f *Fleet) GetParams(pattern, param string, timeout time.Duration) ([]*Device, error) {
	// set parameter
	table, err := fleet.GetParams(f.Broker, param, BaseTopics(f.FilterDevices(pattern)), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of answering devices
	var answering []*Device

	// update device
	for baseTopic, value := range table {
		device := f.DeviceByBaseTopic(baseTopic)
		if device != nil {
			device.Parameters[param] = value
			answering = append(answering, device)
		}
	}

	return answering, nil
}

// SetParams will set the specified parameter on all devices matching the supplied
// glob pattern. The inventory is updated with the saved value and a list of
// updated devices is returned.
func (f *Fleet) SetParams(pattern, param, value string, timeout time.Duration) ([]*Device, error) {
	// set parameter
	table, err := fleet.SetParams(f.Broker, param, value, BaseTopics(f.FilterDevices(pattern)), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of updated devices
	var updated []*Device

	// update device
	for baseTopic, value := range table {
		device := f.DeviceByBaseTopic(baseTopic)
		if device != nil {
			device.Parameters[param] = value
			updated = append(updated, device)
		}
	}

	return updated, nil
}

// UnsetParams will unset the specified parameter on all devices matching the
// supplied glob pattern. The inventory is updated with the removed value and a
// list of updated devices is returned.
func (f *Fleet) UnsetParams(pattern, param string, timeout time.Duration) ([]*Device, error) {
	// set parameter
	err := fleet.UnsetParams(f.Broker, param, BaseTopics(f.FilterDevices(pattern)), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of updated devices
	var updated []*Device

	// update device
	for _, device := range f.FilterDevices(pattern) {
		delete(device.Parameters, param)
		updated = append(updated, device)
	}

	return updated, nil
}

// Record will enable log recording mode and yield the received log messages
// until the provided channel has been closed.
func (f *Fleet) Record(pattern string, quit chan struct{}, timeout time.Duration, callback func(time.Time, *Device, string)) error {
	return fleet.Record(f.Broker, BaseTopics(f.FilterDevices(pattern)), quit, timeout, func(log *fleet.LogMessage) {
		// call user callback
		if callback != nil {
			callback(log.Time, f.DeviceByBaseTopic(log.BaseTopic), log.Content)
		}
	})
}

// Monitor will monitor the devices that match the supplied glob pattern and
// update the inventory accordingly. The specified callback is called for every
// heartbeat with the update device and the heartbeat available at
// device.LastHeartbeat.
func (f *Fleet) Monitor(pattern string, quit chan struct{}, timeout time.Duration, callback func(*Device, *fleet.Heartbeat)) error {
	return fleet.Monitor(f.Broker, BaseTopics(f.FilterDevices(pattern)), quit, timeout, func(heartbeat *fleet.Heartbeat) {
		// get device
		device, ok := f.Devices[heartbeat.DeviceName]
		if !ok {
			return
		}

		// update fields
		device.Type = heartbeat.DeviceType
		device.FirmwareVersion = heartbeat.FirmwareVersion

		// call user callback
		if callback != nil {
			callback(device, heartbeat)
		}
	})
}

// Debug will load the coredump data from the devices that match the supplied
// glob pattern.
func (f *Fleet) Debug(pattern string, delete bool, duration time.Duration) (map[*Device][]byte, error) {
	// gather coredumps
	coredumps, err := fleet.Debug(f.Broker, BaseTopics(f.FilterDevices(pattern)), delete, duration)
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
		table[f.DeviceByBaseTopic(baseTopic)] = coredump
	}

	return table, nil
}

// Update will update the devices that match the supplied glob pattern with the
// specified image. The specified callback is called for every change in state
// or progress.
func (f *Fleet) Update(version, pattern string, firmware []byte, jobs int, timeout time.Duration, callback func(*Device, *fleet.UpdateStatus)) error {
	// get devices
	list := f.FilterDevices(pattern)

	// prepare devices
	var devices []*Device

	// check version
	for _, d := range list {
		if d.FirmwareVersion != version {
			devices = append(devices, d)
		}
	}

	return fleet.Update(f.Broker, BaseTopics(devices), firmware, jobs, timeout, func(baseTopic string, status *fleet.UpdateStatus) {
		// get device
		device := f.DeviceByBaseTopic(baseTopic)
		if device == nil {
			return
		}

		// call callback
		callback(device, status)
	})
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
