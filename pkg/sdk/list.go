package sdk

import (
	"os"
	"path/filepath"
	"strings"
)

// SDK represents an installed SDK.
type SDK struct {
	Name    string // e.g. esp-idf, tools
	Version string // e.g. v5.3
	Path    string
}

// List will return all installed SDKs.
func List() ([]SDK, error) {
	// get user home directory
	usr, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// prepare directory
	dir := filepath.Join(usr, ".naos", "sdks")

	// entries entries
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// iterate over entries
	var sdks []SDK
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, nil
		}

		// split name
		name := entry.Name()
		parts := strings.Split(name, "+")
		if len(parts) != 2 {
			return nil, nil
		}

		// append sdk
		sdks = append(sdks, SDK{
			Name:    parts[0],
			Version: parts[1],
			Path:    filepath.Join(dir, name),
		})
	}

	return sdks, nil
}
