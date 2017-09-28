package tree

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// IncludeDirectories returns a list of directories that will be included in the
// build process.
func IncludeDirectories(treePath string) ([]string, error) {
	// update includes.list
	err := Exec(treePath, nil, nil, "make", "generate_component_includes")
	if err != nil {
		return nil, err
	}

	// read file
	bytes, err := ioutil.ReadFile(filepath.Join(treePath, "includes.list"))
	if err != nil {
		return nil, err
	}

	// split lines and trim whitespace
	list := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	for i, item := range list {
		list[i] = strings.TrimSpace(item)
	}

	return list, nil
}

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
