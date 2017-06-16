package naos

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"encoding/json"
)

// ConfigName represents the default configuration filename.
const ConfigName = "naos.json"

// A Config represents the contents of the configuration file contained in a NAOS
// project.
type Config struct {
	Name string
}

// ReadConfig will attempt to read the config file at the specified path.
func ReadConfig(path string) (*Config, error) {
	// prepare config
	var config Config

	// read file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// decode data
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// Save will write the config file to the specified path.
func (c *Config) Save(path string) error {
	// encode config
	buf, err := json.Marshal(c)
	if err != nil {
		return err
	}

	// write config
	err = ioutil.WriteFile(path, buf, 0644)
	if err != nil {
		return err
	}

	return nil
}

// A Project is a NAOS project available on disk.
type Project struct {
	Location string
	Config   *Config
}

// CreateProject will initialize a project in the specified directory with the
// specified name.
func CreateProject(path, name string) (*Project, error) {
	// ensure project directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	// create project
	project := &Project{
		Location: path,
		Config: &Config{
			Name: name,
		},
	}

	// save config
	err = project.SaveConfig()
	if err != nil {
		return nil, err
	}

	// TODO: Create "main.c".
	// TODO: Create CMakeLists.txt.

	return project, nil
}

// FindProject will look for NAOS project in the specified path.
func FindProject(path string) (*Project, error) {
	// attempt to read config file
	config, err := ReadConfig(filepath.Join(path, ConfigName))
	if err != nil {
		return nil, err
	}

	// TODO: Walk up the tree and try parent directories.

	// prepare project
	project := &Project{
		Location: path,
		Config:   config,
	}

	return project, nil
}

// SaveConfig will save the projects config.
func (p *Project) SaveConfig() error {
	return p.Config.Save(filepath.Join(p.Location, ConfigName))
}
