package utils

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"go.bug.st/serial"
)

// Port represents a USB serial port.
type Port struct {
	Path string
}

var usbPrefixes = []string{"cu.SLAB", "cu.usbserial", "cu.usbmodem", "ttyUSB"}

// ListPorts will return a list of all known USB serial ports.
func ListPorts() ([]Port, error) {
	// get list
	list, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	// sort in reverse to list combined ports with their serial port first
	sort.Sort(sort.Reverse(sort.StringSlice(list)))

	// check names and prefixes
	ports := make([]Port, 0)
	for _, name := range list {
		for _, prefix := range usbPrefixes {
			if strings.Contains(name, prefix) {
				ports = append(ports, Port{
					Path: name,
				})
			}
		}
	}

	return ports, nil
}

// FindPort will return the fist known USB serial port or an empty string.
func FindPort(out io.Writer) string {
	// list ports
	ports, err := ListPorts()
	if err != nil {
		Log(out, fmt.Sprintf("usb: %s\n", err.Error()))
		return ""
	}

	// return first port
	if len(ports) > 0 {
		return ports[0].Path
	}

	return ""
}
