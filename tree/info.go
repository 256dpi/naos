package tree

import (
	"os"
	"path/filepath"
)

// SourceAndHeaderFiles will return a list of source and header files.
func SourceAndHeaderFiles(treePath string) ([]string, []string, error) {
	// prepare list
	sourceFiles := make([]string, 0)
	headerFiles := make([]string, 0)

	// scan directory
	err := filepath.Walk(filepath.Join(treePath, "main", "src"), func(path string, f os.FileInfo, err error) error {
		// directly return errors
		if err != nil {
			return err
		}

		// add files with matching extension
		if filepath.Ext(path) == ".c" {
			sourceFiles = append(sourceFiles, path)
		} else if filepath.Ext(path) == ".h" {
			headerFiles = append(headerFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sourceFiles, headerFiles, nil
}
