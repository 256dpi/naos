package naos

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/256dpi/naos/pkg/tree"
)

// A Component represents an installable component.
type Component struct {
	Path       string `json:"path"`
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Version    string `json:"version"`
}

// Frameworks represents the official frameworks.
type Frameworks struct {
	Audio string `json:"audio,omitempty"`
}

// An Inventory represents the contents of the inventory file.
type Inventory struct {
	Version    string                `json:"version"`
	Target     string                `json:"target"`
	BaudRate   string                `json:"baud_rate,omitempty"`
	Embeds     []string              `json:"embeds"`
	Overrides  map[string]string     `json:"overrides"`
	Partitions *tree.Partitions      `json:"partitions"`
	Components map[string]*Component `json:"components"`
	Frameworks Frameworks            `json:"frameworks,omitempty"`
}

// NewInventory creates a new Inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Version:    "master",
		Target:     "esp32",
		Embeds:     make([]string, 0),
		Overrides:  make(map[string]string),
		Components: make(map[string]*Component),
	}
}

// ReadInventory will attempt to read the inventory file at the specified path.
func ReadInventory(path string) (*Inventory, error) {
	// prepare inventory
	var inv Inventory

	// read file
	data, err := os.ReadFile(path)
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
	err = os.WriteFile(path, append(data, '\n'), 0644)
	if err != nil {
		return err
	}

	return nil
}
