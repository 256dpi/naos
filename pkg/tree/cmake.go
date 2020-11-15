package tree

import (
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// WriteCMakeLists will write a CMakeLists.txt with all included directories.
func WriteCMakeLists(naosPath string, out io.Writer) error {
	// log message
	utils.Log(out, "Generating CMakeLists.txt...")

	// get list of include directories
	list, err := IncludeDirectories(naosPath)
	if err != nil {
		return err
	}

	// prepare file
	file := "target_include_directories(${CMAKE_PROJECT_NAME} PUBLIC"

	// add includes
	for _, item := range list {
		file += "\n        " + item
	}

	// add delimiter
	file += ")\n"

	// update cmake config
	err = ioutil.WriteFile(filepath.Join(naosPath, "CMakeLists.txt"), []byte(file), 0644)
	if err != nil {
		return err
	}

	return nil
}
