// Package tree provides utility functions to manage the NAOS build tree.
package tree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// Install will install the NAOS repo to the specified path and link the source
// path into the build tree.
func Install(naosPath, sourcePath, dataPath, version string, force, fixSerial bool, out io.Writer) error {
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
		err = utils.Clone("https://github.com/256dpi/naos.git", naosPath, version, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating NAOS '%s'...", version))
		err = utils.Fetch(naosPath, version, out)
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

	// install toolchain for v4 projects
	if idfMajorVersion == 4 {
		err = InstallToolchain4(naosPath, force, out)
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

	// fix serial if requested
	if fixSerial {
		utils.Log(out, "Fixing serial.")
		esptoolPath := filepath.Join(Directory(naosPath), "esp-idf", "components", "esptool_py", "esptool")
		err = utils.Replace(esptoolPath, "https://github.com/256dpi/esptool.git", "fork", "dtr-rts-fix", out)
		if err != nil {
			return err
		}
	}

	return nil
}

// InstallComponent will install the specified component in the build tree.
func InstallComponent(projectPath, naosPath, name, path, repository, version string, force bool, out io.Writer) error {
	// check component name
	if name == "esp-mqtt" || name == "naos" {
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
		err = utils.Clone(repository, comPath, version, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating component '%s' '%s'...", name, version))
		err = utils.Fetch(comPath, version, out)
		if err != nil {
			return err

		}
	}

	return nil
}
