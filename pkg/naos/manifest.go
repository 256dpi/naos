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

// A Manifest represents the contents of the manifest file.
type Manifest struct {
	Name       string                `json:"name"`
	Version    string                `json:"version"`
	Target     string                `json:"target"`
	BaudRate   string                `json:"baud_rate,omitempty"`
	TagPrefix  string                `json:"tag_prefix,omitempty"`
	Embeds     []string              `json:"embeds"`
	Overrides  map[string]string     `json:"overrides"`
	Partitions *tree.Partitions      `json:"partitions"`
	Components map[string]*Component `json:"components"`
	Frameworks Frameworks            `json:"frameworks,omitempty"`
}

// NewManifest creates a new Manifest.
func NewManifest() *Manifest {
	return &Manifest{
		Version:    "main",
		Name:       "my-device",
		Target:     "esp32",
		Embeds:     make([]string, 0),
		Overrides:  make(map[string]string),
		Components: make(map[string]*Component),
	}
}

// ReadManifest will attempt to read the manifest file at the specified path.
func ReadManifest(path string) (*Manifest, error) {
	// prepare manifest
	var man Manifest

	// read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// decode data
	err = json.Unmarshal(data, &man)
	if err != nil {
		return nil, err
	}

	// create map of components if missing
	if man.Components == nil {
		man.Components = make(map[string]*Component)
	}

	// check version
	if man.Version == "" {
		return nil, errors.New("missing version field")
	}

	return &man, nil
}

// Save will write the manifest file to the specified path.
func (m *Manifest) Save(path string) error {
	// encode data
	data, err := json.MarshalIndent(m, "", "  ")
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
