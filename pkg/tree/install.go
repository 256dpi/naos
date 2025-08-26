// Package tree provides utility functions to manage the NAOS build tree.
package tree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/256dpi/naos/pkg/sdk"
	"github.com/256dpi/naos/pkg/utils"
)

// https://github.com/espressif/idf-component-manager

type idfComponentFile struct {
	Dependencies map[string]idfComponentItem `yaml:"dependencies"`
}

type idfComponentItem struct {
	Version string             `yaml:"version"`
	Git     string             `yaml:"git,omitempty"`
	Path    string             `yaml:"path,omitempty"`
	Rules   []idfComponentRule `yaml:"rules"`
}

type idfComponentRule struct {
	If string `yaml:"if"`
}

// IDFComponent represents a component in the idf-components.yml file.
type IDFComponent struct {
	Name       string
	Version    string
	Repository string
	Path       string
}

// Install will install the NAOS repo to the specified path and link the source
// path into the build tree.
func Install(naosPath, sourcePath, dataPath, version string, force bool, out io.Writer) error {
	// remove existing directory if existing or force has been set
	if force {
		utils.Log(out, "Removing existing NAOS installation (forced).")
		err := os.RemoveAll(naosPath)
		if err != nil {
			return err
		}
	}

	// check if tree already exists
	ok, err := utils.Exists(naosPath)
	if err != nil {
		return err
	}

	// check existence
	if !ok {
		// perform initial repo clone
		utils.Log(out, fmt.Sprintf("Installing NAOS '%s'...", version))
		err = utils.Clone("https://github.com/256dpi/naos.git", naosPath, version, []string{
			"tree/esp-idf",
		}, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating NAOS '%s'...", version))
		err = utils.Fetch(naosPath, version, []string{
			"tree/esp-idf",
		}, out)
		if err != nil {
			return err
		}
	}

	// get IDF version
	idfVersion, err := IDFVersion(naosPath)
	if err != nil {
		return err
	}

	// install IDF
	utils.Log(out, fmt.Sprintf("Installing ESP-IDF '%s'...", idfVersion))
	idf, err := sdk.InstallIDF(idfVersion, out)
	if err != nil {
		return err
	}

	// link IDF
	utils.Log(out, "Linking ESP-IDF.")
	err = utils.Link(filepath.Join(Directory(naosPath), "esp-idf"), idf)
	if err != nil {
		return err
	}

	// install toolchain
	toolchain, err := sdk.InstallToolchain(idfVersion, out)
	if err != nil {
		return err
	}

	// remove old toolchain
	oldToolchain, err := utils.IsDir(filepath.Join(Directory(naosPath), "toolchain"))
	if err != nil {
		return err
	} else if oldToolchain {
		utils.Log(out, "Removing old toolchain.")
		err = os.RemoveAll(filepath.Join(Directory(naosPath), "toolchain"))
		if err != nil {
			return err
		}
	}

	// link toolchain
	utils.Log(out, "Linking toolchain.")
	err = utils.Link(filepath.Join(Directory(naosPath), "toolchain"), toolchain)
	if err != nil {
		return err
	}

	// link source directory if missing
	utils.Log(out, "Linking source directory.")
	err = utils.Link(filepath.Join(Directory(naosPath), "main", "src"), sourcePath)
	if err != nil {
		return err
	}

	// link data directory if missing
	utils.Log(out, "Linking data directory.")
	err = utils.Link(filepath.Join(Directory(naosPath), "main", "data"), dataPath)
	if err != nil {
		return err
	}

	return nil
}

// InstallComponent will install the specified component in the build tree.
func InstallComponent(projectPath, naosPath, name, path, repository, version string, force bool, out io.Writer) error {
	// check component name
	if name == "esp-mqtt" || name == "esp-osc" || name == "naos" {
		return errors.New("illegal component name")
	}

	// get component dir
	comPath := filepath.Join(Directory(naosPath), "components", name)

	// remove existing directory if existing or force has been set
	if force {
		utils.Log(out, fmt.Sprintf("Removing existing component '%s' (forced).", name))
		err := os.RemoveAll(comPath)
		if err != nil {
			return err
		}
	}

	// handle local links
	if path != "" {
		// resolve target
		target, err := utils.Resolve(path, projectPath)
		if err != nil {
			return err
		}

		// ensure link
		err = utils.Link(comPath, target)
		if err != nil {
			return err
		}

		return nil
	}

	// check if component already exists
	ok, err := utils.Exists(comPath)
	if err != nil {
		return err
	}

	// check existence
	if !ok {
		// perform initial repo clone
		utils.Log(out, fmt.Sprintf("Installing component '%s' '%s'...", name, version))
		err = utils.Clone(repository, comPath, version, nil, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating component '%s' '%s'...", name, version))
		err = utils.Fetch(comPath, version, nil, out)
		if err != nil {
			return err
		}
	}

	return nil
}

// InstallRegistryComponents will install the specified registry components.
func InstallRegistryComponents(projectPath, naosPath string, components []IDFComponent, force bool, out io.Writer) error {
	// load original file
	original, err := utils.Original(naosPath, filepath.Join("tree", "main", "idf_component.yml"))
	if err != nil {
		return err
	}

	// unmarshal original file
	var file idfComponentFile
	err = yaml.Unmarshal([]byte(original), &file)
	if err != nil {
		return err
	}

	// ensure map
	if file.Dependencies == nil {
		file.Dependencies = make(map[string]idfComponentItem)
	}

	// add components
	for _, c := range components {
		file.Dependencies[c.Name] = idfComponentItem{
			Version: c.Version,
			Git:     c.Repository,
			Path:    c.Path,
		}
	}

	// marshal updated file
	updated, err := yaml.Marshal(file)
	if err != nil {
		return err
	}

	// update file
	err = utils.Update(filepath.Join(Directory(naosPath), "main", "idf_component.yml"), string(updated))
	if err != nil {
		return err
	}

	// update dependencies
	utils.Log(out, "Updating IDF dependencies...")
	err = Exec(naosPath, out, nil, false, false, "idf.py", "update-dependencies")
	if err != nil {
		return err
	}

	return nil
}

// InstallAudioFramework will manage the esp-adf installation.
func InstallAudioFramework(naosPath, version string, force bool, out io.Writer) error {
	// check existences
	exists, err := utils.Exists(ADFDirectory(naosPath))
	if err != nil {
		return err
	}

	// handle removal
	if version == "" {
		if exists {
			utils.Log(out, fmt.Sprintf("Removing audio framework..."))
			err := os.RemoveAll(ADFDirectory(naosPath))
			if err != nil {
				return err
			}
		}
		return nil
	}

	/* otherwise, handle install */

	// remove existing directory if force has been set
	if force && exists {
		utils.Log(out, fmt.Sprintf("Removing existing audio framework (forced)."))
		err := os.RemoveAll(ADFDirectory(naosPath))
		if err != nil {
			return err
		}
	}

	// check existence
	if !exists {
		// perform initial repo clone
		utils.Log(out, fmt.Sprintf("Installing audio framework '%s'...", version))
		err = utils.Clone("https://github.com/espressif/esp-adf.git", ADFDirectory(naosPath), version, []string{"esp-idf"}, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating audio framework '%s'...", version))
		err = utils.Fetch(ADFDirectory(naosPath), version, []string{"esp-idf"}, out)
		if err != nil {
			return err

		}
	}

	/* apply ADF patches */

	// get version
	idfVersion, err := IDFVersion(naosPath)
	if err != nil {
		return err
	}

	// determine patches
	var patches []string
	if strings.HasPrefix(idfVersion, "v4.4") {
		patches = append(patches, "idf_v4.4_freertos.patch")
	}

	// apply patches
	if len(patches) > 0 {
		for _, patch := range patches {
			utils.Log(out, fmt.Sprintf("Applying audio framework patch '%s'...", patch))
			err = utils.Apply(IDFDirectory(naosPath), filepath.Join(ADFDirectory(naosPath), "idf_patches", patch), out)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
