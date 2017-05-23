package main

import (
	"fmt"
	"os"
)

func exitIfSet(errs ...error) {
	for _, err := range errs {
		if err != nil {
			exitWithError(err.Error())
		}
	}
}

func exitWithError(str string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", str)
	os.Exit(1)
}
