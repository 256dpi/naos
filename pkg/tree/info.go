package tree

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// IncludeDirectories returns a list of directories that will be included in the
// build process.
func IncludeDirectories(naosPath string) ([]string, error) {
	// update includes.list
	err := Exec(naosPath, nil, nil, "make", "generate_component_includes")
	if err != nil {
		return nil, err
	}

	// read file
	bytes, err := ioutil.ReadFile(filepath.Join(Directory(naosPath), "includes.list"))
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

// RequiredToolchain returns the required toolchain version by the current
// NAOS installation.
func RequiredToolchain(naosPath string) (string, error) {
	// read version
	bytes, err := ioutil.ReadFile(filepath.Join(Directory(naosPath), "toolchain.version"))
	if err != nil {
		return "", err
	}

	// trim version
	version := strings.TrimSpace(string(bytes))

	// check version
	if version == "" || version == "-" {
		return "", errors.New("malformed version")
	}

	return string(version), nil
}

// SourceAndHeaderFiles will return a list of source and header files.
func SourceAndHeaderFiles(naosPath string) ([]string, []string, error) {
	// prepare list
	sourceFiles := make([]string, 0)
	headerFiles := make([]string, 0)

	// read link
	path, err := os.Readlink(filepath.Join(Directory(naosPath), "main", "src"))
	if err != nil {
		return nil, nil, err
	}

	// scan directory
	err = filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
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
