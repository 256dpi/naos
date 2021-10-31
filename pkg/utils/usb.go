package utils

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"go.bug.st/serial"
)

var usbPrefixes = []string{"cu.SLAB", "cu.usbserial", "ttyUSB"}

// FindPort will return the fist known USB serial port or an empty string.
func FindPort(out io.Writer) string {
	// get list
	list, err := serial.GetPortsList()
	if err != nil {
		_, _ = fmt.Fprintf(out, "usb: %s\n", err.Error())
		return ""
	}

	// sort list
	sort.Strings(list)

	// check names and prefixes
	for _, name := range list {
		for _, prefix := range usbPrefixes {
			if strings.Contains(name, prefix) {
				return name
			}
		}
	}

	return ""
}
