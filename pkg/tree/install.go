// Package tree provides utility functions to manage the NAOS build tree.
package tree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

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
		err = utils.Clone("https://github.com/256dpi/naos.git", naosPath, version, nil, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating NAOS '%s'...", version))
		err = utils.Fetch(naosPath, version, nil, out)
		if err != nil {
			return err
		}
	}

	// get major IDF version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// install toolchain for v3 projects
	if idfMajorVersion == 3 {
		// get required toolchain
		toolchainVersion, err := RequiredToolchain(naosPath)
		if err != nil {
			return err
		}

		// install toolchain
		err = InstallToolchain3(naosPath, toolchainVersion, force, out)
		if err != nil {
			return err
		}
	}

	// install toolchain for new projects
	if idfMajorVersion >= 4 {
		err = InstallToolchain(naosPath, force, out)
		if err != nil {
			return err
		}
	}

	// link source directory if missing
	ok, err = utils.Exists(filepath.Join(Directory(naosPath), "main", "src"))
	if err != nil {
		return err
	} else if !ok {
		utils.Log(out, "Linking source directory.")
		err = os.Symlink(sourcePath, filepath.Join(Directory(naosPath), "main", "src"))
		if err != nil {
			return err
		}
	}

	// link data directory if missing
	ok, err = utils.Exists(filepath.Join(Directory(naosPath), "main", "data"))
	if err != nil {
		return err
	} else if !ok {
		utils.Log(out, "Linking data directory.")
		err = os.Symlink(dataPath, filepath.Join(Directory(naosPath), "main", "data"))
		if err != nil {
			return err
		}
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
