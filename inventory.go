package nadm

import (
	"encoding/json"
	"io/ioutil"

	"github.com/ryanuber/go-glob"
)

// A Inventory represents the contents of the inventory file.
type Inventory struct {
	Broker  string            `json:"broker"`
	Devices map[string]string `json:"devices"`
}

// NewInventory creates a new Inventory.
func NewInventory(broker string) *Inventory {
	return &Inventory{
		Broker: broker,
		Devices: make(map[string]string),
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
		inv.Devices = make(map[string]string)
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

// GetName will return the name of the device that matches the supplied base
// topic or an empty string if the device has not been found.
func (i *Inventory) GetName(baseTopic string) string {
	// go over all devices
	for n, bt := range i.Devices {
		if bt == baseTopic {
			return n
		}
	}

	return ""
}

// GetBaseTopic will return the base topic for the given device or an empty
// string if the device has not been found.
func (i *Inventory) GetBaseTopic(deviceName string) string {
	// go over all devices
	for name, baseTopic := range i.Devices {
		if name == deviceName {
			return baseTopic
		}
	}

	return ""
}

// GetNames will return a list of device names that match the supplied
// glob pattern.
func (i *Inventory) GetNames(filter string) []string {
	// prepare list
	var names []string

	// go over all devices
	for name := range i.Devices {
		// add name if it matches glob
		if glob.Glob(filter, name) {
			names = append(names, name)
		}
	}

	return names
}

// GetBaseTopics will return a list of base topics for the devices that match
// the supplied glob pattern.
func (i *Inventory) GetBaseTopics(filter string) []string {
	// prepare list
	var baseTopics []string

	// go over all devices
	for name, baseTopic := range i.Devices {
		// add base topic if name matches glob
		if glob.Glob(filter, name) {
			baseTopics = append(baseTopics, baseTopic)
		}
	}

	return baseTopics
}
