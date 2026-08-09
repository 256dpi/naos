package serial

import (
	"errors"
	"sort"
	"strings"

	"go.bug.st/serial"
)

var knownPrefixes = []string{"cu.SLAB", "cu.usbserial", "cu.usbmodem", "ttyUSB"}

// ListPorts will return a list of all known serial ports.
func ListPorts() ([]string, error) {
	// get list
	list, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	// sort in reverse to list combined ports with their serial port first
	sort.Sort(sort.Reverse(sort.StringSlice(list)))

	// check names and prefixes
	ports := make([]string, 0)
	for _, name := range list {
		for _, prefix := range knownPrefixes {
			if strings.Contains(name, prefix) {
				ports = append(ports, name)
			}
		}
	}

	return ports, nil
}

// FindPort will return the first known USB serial port.
func FindPort() (string, error) {
	// list ports
	ports, err := ListPorts()
	if err != nil {
		return "", err
	}

	// check list
	if len(ports) == 0 {
		return "", errors.New("no serial device found")
	}

	return ports[0], nil
}
