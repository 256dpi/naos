package main

import (
	"fmt"
	"os"

	"github.com/shiftr-io/nadm/fleet"
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

func getInventory(cmd *command) *fleet.Inventory {
	inv, err := fleet.ReadInventory(cmd.oInventory)
	exitIfSet(err)
	return inv
}
