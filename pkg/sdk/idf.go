package sdk

import (
	"io"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// InstallIDF will install the specified version of the ESP-IDF SDK.
func InstallIDF(version string, out io.Writer) (string, error) {
	// get user home directory
	usr, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// prepare base directory
	base := filepath.Join(usr, ".naos", "sdks")

	// ensure directory
	err = os.MkdirAll(base, 0755)
	if err != nil {
		return "", err
	}

	// prepare key
	key := "esp-idf+" + version

	// prepare directory
	dir := filepath.Join(base, key)

	// check existence
	ok, err := utils.Exists(dir)
	if err != nil {
		return "", err
	}

	// clone or fetch
	if !ok {
		err = utils.Clone("https://github.com/espressif/esp-idf.git", dir, version, nil, out)
		if err != nil {
			return "", err
		}
	} else {
		err = utils.Fetch(dir, version, nil, out)
		if err != nil {
			return "", err
		}
	}

	return dir, nil
}
