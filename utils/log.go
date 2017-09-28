// Package utils provides some small utility functions.
package utils

import (
	"fmt"
	"io"
)

// Log will format and write the provided message to out if available.
func Log(out io.Writer, msg string) {
	if out != nil {
		fmt.Fprintf(out, "==> %s\n", msg)
	}
}
