package main

import (
	"fmt"
	"os"

	"github.com/shiftr-io/nadm"
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

func getInventory(cmd *command) *nadm.Inventory {
	inv, err := nadm.ReadInventory(cmd.oInventory)
	exitIfSet(err)
	return inv
}
