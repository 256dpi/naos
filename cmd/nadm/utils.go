package main

import (
	"fmt"
	"os"
	"os/signal"
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

func waitForInterrupt() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
