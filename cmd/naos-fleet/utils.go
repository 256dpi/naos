package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/fleet"
)

func exitIfSet(errs ...error) {
	for _, err := range errs {
		if err != nil {
			exitWithError(err.Error())
		}
	}
}

func exitWithError(str string) {
	_, _ = fmt.Fprintf(os.Stderr, "Error: %s\n", str)
	os.Exit(1)
}

func workingDirectory() string {
	wd, err := os.Getwd()
	exitIfSet(err)

	return wd
}

func fleetFilePath() string {
	return filepath.Join(workingDirectory(), "fleet.json")
}

func getFleet() *fleet.Fleet {
	f, err := fleet.ReadFleet(fleetFilePath())
	exitIfSet(err)

	return f
}

func saveFleet(f *fleet.Fleet) {
	err := f.Save(fleetFilePath())
	exitIfSet(err)
}
