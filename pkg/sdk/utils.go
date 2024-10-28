package sdk

import (
	"os"
	"path/filepath"
)

func ensureBase() (string, error) {
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

	return base, nil
}
