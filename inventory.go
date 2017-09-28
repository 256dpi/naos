package naos

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"

	"github.com/ryanuber/go-glob"
	"github.com/shiftr-io/naos/mqtt"
)

// A Device represents a single device in an Inventory.
type Device struct {
	BaseTopic       string            `json:"base_topic"`
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	FirmwareVersion string            `json:"firmware_version"`
	Parameters      map[string]string `json:"parameters"`
}

// A Component represents an installable naos component.
type Component struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
}

// A Inventory represents the contents of the inventory file.
type Inventory struct {
	Version    string                `json:"version"`
	Components map[string]*Component `json:"components"`
	Broker     string                `json:"broker"`
	Devices    map[string]*Device    `json:"devices"`
}

// NewInventory creates a new Inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Version:    "master",
		Components: make(map[string]*Component),
		Broker:     "mqtts://key:secret@broker.shiftr.io",
		Devices:    make(map[string]*Device),
	}
}

// ReadInventory will attempt to read the inventory file at the specified path.
func ReadInventory(path string) (*Inventory, error) {
	// prepare inventory
	var inv Inventory

	// read file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// decode data
	err = json.Unmarshal(data, &inv)
	if err != nil {
		return nil, err
	}

	// create map of components if missing
	if inv.Components == nil {
		inv.Components = make(map[string]*Component)
	}

	// create map of devices if missing
	if inv.Devices == nil {
		inv.Devices = make(map[string]*Device)
	}

	// iterate over all devices and initialize parameters
	for _, device := range inv.Devices {
		if device.Parameters == nil {
			device.Parameters = make(map[string]string)
		}
	}

	// check version
	if inv.Version == "" {
		return nil, errors.New("missing version field")
	}

	return &inv, nil
}

// Save will write the inventory file to the specified path.
func (i *Inventory) Save(path string) error {
	// encode data
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}

	// write config
	err = ioutil.WriteFile(path, append(data, '\n'), 0644)
	if err != nil {
		return err
	}

	return nil
}

// FilterDevices will return a list of devices that have a name matching the supplied
// glob pattern.
func (i *Inventory) FilterDevices(pattern string) []*Device {
	// prepare list
	var devices []*Device

	// go over all devices
	for name, device := range i.Devices {
		// add name if it matches glob
		if glob.Glob(pattern, name) {
			devices = append(devices, device)
		}
	}

	return devices
}

// DeviceBaseTopics returns a list of base topics from devices that match the
// supplied pattern.
func (i *Inventory) DeviceBaseTopics(pattern string) []string {
	// prepare list
	var l []string

	// add all matching devices
	for _, d := range i.FilterDevices(pattern) {
		l = append(l, d.BaseTopic)
	}

	return l
}

// DeviceByBaseTopic returns the first device that has the matching base topic.
func (i *Inventory) DeviceByBaseTopic(baseTopic string) *Device {
	// iterate through all devices
	for _, d := range i.Devices {
		if d.BaseTopic == baseTopic {
			return d
		}
	}

	return nil
}

// Collect will collect announcements and update the inventory with found devices
// for the given amount of time. It will return a list of devices that have been
// added to the inventory.
func (i *Inventory) Collect(duration time.Duration) ([]*Device, error) {
	// collect announcements
	anns, err := mqtt.Collect(i.Broker, duration)
	if err != nil {
		return nil, err
	}

	// prepare list
	var newDevices []*Device

	// handle all announcements
	for _, a := range anns {
		// get current device or add one if not existing
		d, ok := i.Devices[a.DeviceName]
		if !ok {
			d = &Device{Name: a.DeviceName, Parameters: make(map[string]string)}
			i.Devices[a.DeviceName] = d
			newDevices = append(newDevices, d)
		}

		// update fields
		d.BaseTopic = a.BaseTopic
		d.Type = a.DeviceType
		d.FirmwareVersion = a.FirmwareVersion
	}

	return newDevices, nil
}

// GetParams will request specified parameter from all devices matching the supplied
// glob pattern. The inventory is updated with the reported value and a list of
// answering devices is returned.
func (i *Inventory) GetParams(pattern, param string, timeout time.Duration) ([]*Device, error) {
	// set parameter
	table, err := mqtt.GetParams(i.Broker, param, i.DeviceBaseTopics(pattern), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of answering devices
	var answering []*Device

	// update device
	for baseTopic, value := range table {
		device := i.DeviceByBaseTopic(baseTopic)
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
func (i *Inventory) SetParams(pattern, param, value string, timeout time.Duration) ([]*Device, error) {
	// set parameter
	table, err := mqtt.SetParams(i.Broker, param, value, i.DeviceBaseTopics(pattern), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of updated devices
	var updated []*Device

	// update device
	for baseTopic, value := range table {
		device := i.DeviceByBaseTopic(baseTopic)
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
func (i *Inventory) UnsetParams(pattern, param string, timeout time.Duration) ([]*Device, error) {
	// set parameter
	err := mqtt.UnsetParams(i.Broker, param, i.DeviceBaseTopics(pattern), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of updated devices
	var updated []*Device

	// update device
	for _, device := range i.FilterDevices(pattern) {
		delete(device.Parameters, param)
		updated = append(updated, device)
	}

	return updated, nil
}

// Record will enable log recording mode and yield the received log messages
// until the provided channel has been closed.
func (i *Inventory) Record(pattern string, quit chan struct{}, timeout time.Duration, callback func(*Device, string)) error {
	return mqtt.Record(i.Broker, i.DeviceBaseTopics(pattern), quit, timeout, func(log *mqtt.LogMessage) {
		// call user callback
		if callback != nil {
			callback(i.DeviceByBaseTopic(log.BaseTopic), log.Content)
		}
	})
}

// Monitor will monitor the devices that match the supplied glob pattern and
// update the inventory accordingly. The specified callback is called for every
// heartbeat with the update device and the heartbeat available at
// device.LastHeartbeat.
func (i *Inventory) Monitor(pattern string, quit chan struct{}, timeout time.Duration, callback func(*Device, *mqtt.Heartbeat)) error {
	return mqtt.Monitor(i.Broker, i.DeviceBaseTopics(pattern), quit, timeout, func(heartbeat *mqtt.Heartbeat) {
		// get device
		device, ok := i.Devices[heartbeat.DeviceName]
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

// Update will update the devices that match the supplied glob pattern with the
// specified image. The specified callback is called for every change in state
// or progress.
func (i *Inventory) Update(pattern string, firmware []byte, timeout time.Duration, callback func(*Device, *mqtt.UpdateStatus)) error {
	return mqtt.Update(i.Broker, i.DeviceBaseTopics(pattern), firmware, timeout, func(baseTopic string, status *mqtt.UpdateStatus) {
		// get device
		device := i.DeviceByBaseTopic(baseTopic)
		if device == nil {
			return
		}

		// call callback
		callback(device, status)
	})
}
