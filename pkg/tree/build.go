package tree

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// Build will build the project.
func Build(naosPath string, files []string, clean, appOnly bool, out io.Writer) error {
	// prepare files content
	var filesContent = "COMPONENT_EMBED_FILES :="
	for _, file := range files {
		filesContent += " data/" + file
	}
	filesContent += "\n"

	// update files
	utils.Log(out, "Updating files...")
	err := os.WriteFile(filepath.Join(Directory(naosPath), "main", "files.mk"), []byte(filesContent), 0644)
	if err != nil {
		return err
	}

	// sync partitions
	utils.Log(out, "Sync partitions...")
	partSrc := filepath.Join(Directory(naosPath), "main", "data", "partitions.csv")
	partDst := filepath.Join(Directory(naosPath), "partitions.csv")
	err = utils.Sync(partSrc, partDst)
	if err != nil {
		return err
	}

	// clean project if requested
	if clean {
		utils.Log(out, "Cleaning project...")
		err = Exec(naosPath, out, nil, "make", "clean")
		if err != nil {
			return err
		}
	}

	// build project (app only)
	if appOnly {
		utils.Log(out, "Building project (app only)...")
		err = Exec(naosPath, out, nil, "make", "app")
		if err != nil {
			return err
		}

		return nil
	}

	// build project
	utils.Log(out, "Building project...")
	err = Exec(naosPath, out, nil, "make", "all")
	if err != nil {
		return err
	}

	return nil
}

// AppBinary will return the bytes of the built app binary.
func AppBinary(naosPath string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(Directory(naosPath), "build", "naos-project.bin"))
}
