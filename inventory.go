package nadm

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/ryanuber/go-glob"
)

// A Device represents a single device in an Inventory.
type Device struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	BaseTopic string `json:"base_topic"`
}

// A Inventory represents the contents of the inventory file.
type Inventory struct {
	Broker  string             `json:"broker"`
	Devices map[string]*Device `json:"devices"`
}

// NewInventory creates a new Inventory.
func NewInventory(broker string) *Inventory {
	return &Inventory{
		Broker:  broker,
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
	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

// Filter will return a list of devices that have a name matching the supplied
// glob pattern.
func (i *Inventory) Filter(pattern string) []*Device {
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

// Collect will collect announcements and update the inventory with new devices
// for the given amount of time.
func (i *Inventory) Collect(duration time.Duration) ([]*Device, error) {
	// collect announcements
	anns, err := CollectAnnouncements(i.Broker, duration)
	if err != nil {
		return nil, err
	}

	// prepare list
	var newDevices []*Device

	// handle all announcements
	for _, a := range anns {
		// get current device or add one if not existing
		d, ok := i.Devices[a.Name]
		if !ok {
			d = &Device{Name: a.Name}
			i.Devices[a.Name] = d
			newDevices = append(newDevices, d)
		}

		// update fields
		d.Type = a.Type
		d.Version = a.Version
		d.BaseTopic = a.BaseTopic
	}

	return newDevices, nil
}
