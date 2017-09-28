package tree

import (
	"io"

	"github.com/shiftr-io/naos/utils"
)

// Build will build the project.
func Build(treePath string, clean, appOnly bool, out io.Writer) error {
	// clean project if requested
	if clean {
		utils.Log(out, "Cleaning project...")
		err := Exec(treePath, out, nil, "make", "clean")
		if err != nil {
			return err
		}
	}

	// build project (app only)
	if appOnly {
		utils.Log(out, "Building project (app only)...")
		err := Exec(treePath, out, nil, "make", "app")
		if err != nil {
			return err
		}

		return nil
	}

	// build project
	utils.Log(out, "Building project...")
	err := Exec(treePath, out, nil, "make", "all")
	if err != nil {
		return err
	}

	return nil
}
