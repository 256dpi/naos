package main

import (
	"fmt"
	"os"

	"github.com/shiftr-io/naos/pkg/naos"
)

func exitIfSet(errs ...error) {
	for _, err := range errs {
		if err != nil {
			exitWithError(err.Error())
		}
	}
}

func exitWithError(str string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", str)
	os.Exit(1)
}

func workingDirectory() string {
	wd, err := os.Getwd()
	exitIfSet(err)

	return wd
}

func getProject() *naos.Project {
	p, err := naos.OpenProject(workingDirectory())
	exitIfSet(err)

	return p
}
