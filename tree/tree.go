// Package tree provides utility functions to manage the the NAOS build tree.
package tree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shiftr-io/naos/utils"
)

// TODO: Implement tree updating.

// Install will install the NAOS build tree to the specified path and link the
// source path into the tree.
func Install(treePath, sourcePath, version string, force bool, out io.Writer) error {
	// check if already exists
	ok, err := utils.Exists(treePath)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		utils.Log(out, "Skipping tree installation as it already exists.")
		return nil
	}

	// remove existing directory if existing
	if ok {
		utils.Log(out, "Removing existing tree installation (forced).")
		err = os.RemoveAll(treePath)
		if err != nil {
			return err
		}
	}

	// clone repo
	utils.Log(out, fmt.Sprintf("Installing tree (%s)...", version))
	err = utils.Clone("https://github.com/shiftr-io/naos-tree.git", treePath, version, out)
	if err != nil {
		return err
	}

	// install xtensa toolchain
	err = InstallXtensa(treePath, force, out)
	if err != nil {
		return err
	}

	// link sourcePath directory
	utils.Log(out, "Linking sourcePath directory.")
	err = os.Symlink(sourcePath, filepath.Join(treePath, "main", "src"))
	if err != nil {
		return err
	}

	return nil
}

// IDFDirectory returns the assumed location of the esp-idf directory.
//
// Note: It will not check if the directory exists.
func IDFDirectory(parent string) string {
	return filepath.Join(parent, "esp-idf")
}
