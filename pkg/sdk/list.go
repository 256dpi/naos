package sdk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
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
			continue
		}

		// split name
		name := entry.Name()
		parts := strings.Split(name, "+")
		if len(parts) != 2 {
			continue
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

// Remove will remove all SDKs installed for the specified version and return
// the removed ones. Removing the version currently linked into a build tree
// requires a reinstall before that tree can be used again.
func Remove(version string, out io.Writer) ([]SDK, error) {
	// list SDKs
	sdks, err := List()
	if err != nil {
		return nil, err
	}

	// remove matching SDKs
	var removed []SDK
	for _, item := range sdks {
		if item.Version != version {
			continue
		}
		utils.Log(out, fmt.Sprintf("Removing '%s' '%s'...", item.Name, item.Version))
		err = os.RemoveAll(item.Path)
		if err != nil {
			return nil, err
		}
		removed = append(removed, item)
	}

	// check result
	if len(removed) == 0 {
		return nil, fmt.Errorf("no SDK installed for version %q", version)
	}

	return removed, nil
}
