// Package tree provides utility functions to manage the NAOS build tree.
package tree

import "path/filepath"

// Directory returns the assumed location of the build tree directory.
//
// Note: It will not check if the directory exists.
func Directory(naosPath string) string {
	return filepath.Join(naosPath, "tree")
}
