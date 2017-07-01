package naos

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/ryanuber/go-glob"
)

// A Device represents a single device in an Inventory.
type Device struct {
	BaseTopic       string            `json:"base_topic"`
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	FirmwareVersion string            `json:"firmware_version"`
	Parameters      map[string]string `json:"parameters"`
}

// A Inventory represents the contents of the inventory file.
type Inventory struct {
	Broker  string             `json:"broker"`
	Devices map[string]*Device `json:"devices"`
}

// NewInventory creates a new Inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Broker:  "mqtts://key:secret@broker.shiftr.io",
		Devices: make(map[string]*Device),
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

	// create map if missing
	if inv.Devices == nil {
		inv.Devices = make(map[string]*Device)
	}

	// iterate over all devices and initialize parameters
	for _, device := range inv.Devices {
		if device.Parameters == nil {
			device.Parameters = make(map[string]string)
		}
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

// Collect will collect announcements and update the inventory with found devices
// for the given amount of time. It will return a list of devices that have been
// added to the inventory.
func (i *Inventory) Collect(duration time.Duration) ([]*Device, error) {
	// collect announcements
	anns, err := Collect(i.Broker, duration)
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
	table, err := GetParams(i.Broker, param, i.deviceBaseTopics(pattern), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of answering devices
	var answering []*Device

	// update device
	for baseTopic, value := range table {
		device := i.deviceByBaseTopic(baseTopic)
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
	table, err := SetParams(i.Broker, param, value, i.deviceBaseTopics(pattern), timeout)
	if err != nil {
		return nil, err
	}

	// prepare list of updated devices
	var updated []*Device

	// update device
	for baseTopic, value := range table {
		device := i.deviceByBaseTopic(baseTopic)
		if device != nil {
			device.Parameters[param] = value
			updated = append(updated, device)
		}
	}

	return updated, nil
}

// Record will enable log recording mode and yield the received log messages
// until the provided channel has been closed.
func (i *Inventory) Record(pattern string, quit chan struct{}, callback func(*Device, string)) error {
	return Record(i.Broker, i.deviceBaseTopics(pattern), quit, func(log *LogMessage) {
		// call user callback
		if callback != nil {
			callback(i.deviceByBaseTopic(log.BaseTopic), log.Content)
		}
	})
}

// Monitor will monitor the devices that match the supplied glob pattern and
// update the inventory accordingly. The specified callback is called for every
// heartbeat with the update device and the heartbeat available at
// device.LastHeartbeat.
func (i *Inventory) Monitor(pattern string, quit chan struct{}, callback func(*Device, *Heartbeat)) error {
	return Monitor(i.Broker, i.deviceBaseTopics(pattern), quit, func(heartbeat *Heartbeat) {
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

// Update will update the devices that match the supplied glob pattern to the
// specified image. The specified callback is called for every change in state
// or progress.
func (i *Inventory) Update(pattern string, firmware []byte, timeout time.Duration, callback func(*Device, *UpdateStatus)) {
	Update(i.Broker, i.deviceBaseTopics(pattern), firmware, timeout, func(baseTopic string, status *UpdateStatus) {
		// get device
		device := i.deviceByBaseTopic(baseTopic)
		if device == nil {
			return
		}

		// call callback
		callback(device, status)
	})
}

func (i *Inventory) deviceBaseTopics(pattern string) []string {
	// prepare list
	var l []string

	// add all matching devices
	for _, d := range i.FilterDevices(pattern) {
		l = append(l, d.BaseTopic)
	}

	return l
}

func (i *Inventory) deviceByBaseTopic(baseTopic string) *Device {
	// iterate through all devices
	for _, d := range i.Devices {
		if d.BaseTopic == baseTopic {
			return d
		}
	}

	return nil
}
