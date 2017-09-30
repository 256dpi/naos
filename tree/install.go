// Package tree provides utility functions to manage the the NAOS build tree.
package tree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shiftr-io/naos/utils"
)

// Install will install the NAOS build tree to the specified path and link the
// source path into the tree.
func Install(treePath, sourcePath, version string, force bool, out io.Writer) error {
	// remove existing directory if existing or force has been set
	if force {
		utils.Log(out, "Removing existing tree installation (forced).")
		err := os.RemoveAll(treePath)
		if err != nil {
			return err
		}
	}

	// check if tree already exists
	ok, err := utils.Exists(treePath)
	if err != nil {
		return err
	}

	// check existence
	if !ok {
		// perform initial repo clone
		utils.Log(out, fmt.Sprintf("Installing tree '%s'...", version))
		err = utils.Clone("https://github.com/shiftr-io/naos-tree.git", treePath, version, out)
		if err != nil {
			return err

		}
	} else {
		// perform repo update
		utils.Log(out, fmt.Sprintf("Updating tree '%s'...", version))
		err = utils.Fetch(treePath, version, out)
		if err != nil {
			return err

		}
	}

	// get required toolchain
	toolchainVersion, err := RequiredToolchain(treePath)
	if err != nil {
		return err
	}

	// install xtensa toolchain
	err = InstallToolchain(treePath, toolchainVersion, force, out)
	if err != nil {
		return err
	}

	// check source directory
	ok, err = utils.Exists(filepath.Join(treePath, "main", "src"))
	if err != nil {
		return err
	}

	// link source directory if missing
	if !ok {
		utils.Log(out, "Linking source directory.")
		err = os.Symlink(sourcePath, filepath.Join(treePath, "main", "src"))
		if err != nil {
			return err
		}
	}

	return nil
}

// InstallComponent will install the specified component in the tree.
func InstallComponent(treePath, name, repository, version string, force bool, out io.Writer) error {
	// check component name
	if name == "esp-mqtt" || name == "naos-esp" {
		return errors.New("illegal component name")
	}

	// get component dir
	comPath := filepath.Join(treePath, "components", name)

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
