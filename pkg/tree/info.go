package tree

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type compileCommand struct {
	Directory string `json:"directory"`
	Command   string `json:"command"`
	File      string `json:"file"`
}

// IDFVersion will detect and return the IDF version from the specified path.
func IDFVersion(naosPath string) (string, error) {
	// read version
	bytes, err := os.ReadFile(filepath.Join(Directory(naosPath), "esp-idf.version"))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(bytes)), nil
}

// IDFMajorVersion will detect and return the major IDF version from the
// specified path.
func IDFMajorVersion(naosPath string) (int, error) {
	// read version
	bytes, err := os.ReadFile(filepath.Join(Directory(naosPath), "esp-idf.version"))
	if err != nil {
		return 0, err
	}

	// parse version
	if strings.HasPrefix(string(bytes), "v3.") {
		return 3, nil
	} else if strings.HasPrefix(string(bytes), "v4.") {
		return 4, nil
	} else if strings.HasPrefix(string(bytes), "v5.") {
		return 5, nil
	}

	return 0, fmt.Errorf("unknown version")
}

// IncludeDirectories returns a list of directories that will be included in the
// build process.
func IncludeDirectories(naosPath string) ([]string, error) {
	// get idf major version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return nil, err
	}

	// handle old projects
	if idfMajorVersion == 3 {
		// update includes.list
		err = Exec(naosPath, nil, nil, false, false, "make", "generate_component_includes")
		if err != nil {
			return nil, err
		}

		// read file
		bytes, err := os.ReadFile(filepath.Join(Directory(naosPath), "includes.list"))
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

	/* handle new projects */

	// reconfigure project
	err = Exec(naosPath, io.Discard, nil, false, false, "idf.py", "reconfigure")
	if err != nil {
		return nil, err
	}

	// read compile commands
	data, err := os.ReadFile(filepath.Join(Directory(naosPath), "build", "compile_commands.json"))
	if err != nil {
		return nil, err
	}

	// parse compile commands
	var compileCommands []compileCommand
	err = json.Unmarshal(data, &compileCommands)
	if err != nil {
		return nil, err
	}

	// find main compile command
	suffix := filepath.Join("tree", "build", "esp-idf", "main")
	var cmd compileCommand
	for _, cmd = range compileCommands {
		if strings.HasSuffix(cmd.Directory, suffix) {
			break
		}
	}

	// collect include options
	var includes []string
	for _, opt := range strings.Split(cmd.Command, " ") {
		if strings.HasPrefix(opt, "-I") {
			includes = append(includes, opt[2:])
		}
	}

	return includes, nil
}

// RequiredToolchain returns the required toolchain version by the current
// NAOS installation.
func RequiredToolchain(naosPath string) (string, error) {
	// read version
	bytes, err := os.ReadFile(filepath.Join(Directory(naosPath), "toolchain.version"))
	if err != nil {
		return "", err
	}

	// trim version
	version := strings.TrimSpace(string(bytes))

	// check version
	if version == "" || version == "-" {
		return "", errors.New("malformed version")
	}

	return version, nil
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
