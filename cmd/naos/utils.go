package main

import (
	"fmt"
	"io"
	"os"

	"github.com/shiftr-io/naos"
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

func home() string {
	wd, err := os.Getwd()
	exitIfSet(err)

	return wd
}

func getProject(cmd *command) *naos.Project {
	p, err := naos.FindProject(home())
	exitIfSet(err)

	return p
}

func getOutput(cmd *command) io.Writer {
	if cmd.oVerbose {
		return os.Stdout
	}

	return nil
}
