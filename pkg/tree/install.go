// Package tree provides utility functions to manage the the NAOS build tree.
package tree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/256dpi/naos/pkg/utils"
)

// Install will install the NAOS repo to the specified path and link the source
// path into the build tree.
func Install(naosPath, sourcePath, dataPath, version string, overrides map[string]string, force bool, out io.Writer) error {
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

	// get required toolchain
	toolchainVersion, err := RequiredToolchain(naosPath)
	if err != nil {
		return err
	}

	// install xtensa toolchain
	err = InstallToolchain(naosPath, toolchainVersion, force, out)
	if err != nil {
		return err
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

	// apply overrides
	if len(overrides) > 0 {
		utils.Log(out, "Overriding sdkconfig...")

		// determine path
		configPath := filepath.Join(Directory(naosPath), "sdkconfig")

		// read config
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}

		// get config
		sdkconfig := string(data)

		// append comments
		sdkconfig += "\n#\n# OVERRIDES\n#\n"

		// replace lines
		for key, value := range overrides {
			re := regexp.MustCompile("(?m)^(" + regexp.QuoteMeta(key) + ")=(.*)$")
			if re.MatchString(sdkconfig) {
				sdkconfig = re.ReplaceAllString(sdkconfig, key+"="+value)
			} else {
				sdkconfig += key + "=" + value + "\n"
			}
		}

		// write config
		err = os.WriteFile(configPath, []byte(sdkconfig), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// InstallComponent will install the specified component in the build tree.
func InstallComponent(naosPath, name, repository, version string, force bool, out io.Writer) error {
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
