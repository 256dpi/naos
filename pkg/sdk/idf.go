package sdk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

// InstallIDF will install the specified version of the ESP-IDF SDK.
func InstallIDF(version string, out io.Writer) (string, error) {
	// ensure base
	base, err := ensureBase()
	if err != nil {
		return "", err
	}

	// prepare key
	key := "esp-idf+" + version

	// prepare directory
	dir := filepath.Join(base, key)

	// prepare marker
	marker := filepath.Join(dir, ".naos-version")

	// check existence
	ok, err := utils.Exists(dir)
	if err != nil {
		return "", err
	}

	// check if already installed
	if ok {
		data, err := os.ReadFile(marker)
		if err == nil && strings.TrimSpace(string(data)) == version {
			return dir, nil
		}
	}

	// remove existing directory if present
	if ok {
		utils.Log(out, "Removing existing SDK...")
		err = os.RemoveAll(dir)
		if err != nil {
			return "", err
		}
	}

	// prepare zip path
	zipPath := filepath.Join(base, key+".zip")

	// download zip archive
	url := fmt.Sprintf("https://github.com/espressif/esp-idf/releases/download/%s/esp-idf-%s.zip", version, version)
	utils.Log(out, fmt.Sprintf("Downloading '%s'...", url))
	err = utils.Download(zipPath, url)
	if err != nil {
		return "", err
	}

	// extract zip archive
	utils.Log(out, "Extracting archive...")
	err = utils.Unzip(zipPath, base, out)
	if err != nil {
		os.Remove(zipPath)
		return "", err
	}

	// remove zip file
	os.Remove(zipPath)

	// rename extracted directory
	extracted := filepath.Join(base, fmt.Sprintf("esp-idf-%s", version))
	err = os.Rename(extracted, dir)
	if err != nil {
		return "", err
	}

	// write marker
	err = os.WriteFile(marker, []byte(version), 0644)
	if err != nil {
		return "", err
	}

	return dir, nil
}
