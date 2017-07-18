// Package espidf provides utility functions to manage the esp-idf esp32
// development framework needed to build NAOS projects.
package espidf

import (
	"io"
	"os"

	"path/filepath"
	"github.com/shiftr-io/naos/utils"
	"fmt"
)

// Install will install the esp-idf development framework. An existing development
// framework will be removed if force is set to true. If out is not nil, it will
// be used to log information about the installation process.
func Install(parent string, version string, force bool, out io.Writer) error {
	// prepare directory
	dir := filepath.Join(parent, "esp-idf")

	// check if already exists
	ok, err := utils.Exists(dir)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		utils.Log(out, "Skipping esp-idf installation as it already exists.")
		return nil
	}

	// remove existing directory if existing
	if ok {
		utils.Log(out, "Removing existing esp-idf installation (forced).")
		err = os.RemoveAll(dir)
		if err != nil {
			return err
		}
	}

	// clone
	utils.Log(out, fmt.Sprintf("Installing esp-idf (%s)...", version))
	err = utils.Clone("https://github.com/espressif/esp-idf.git", dir, version, out)
	if err != nil {
		return err
	}

	return nil
}

// Directory returns the assumed location of the esp-idf directory.
//
// Note: It will not check if the directory exists.
func Directory(parent string) string {
	return filepath.Join(parent, "esp-idf")
}
