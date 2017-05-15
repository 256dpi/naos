package main

import "github.com/docopt/docopt-go"

// TODO: Url as option and populate from nadm.rc.

var usage = `nadm - the networked artifacts device manager

Usage:
  nadm collect [--broker=<broker>]
  nadm update <base-topic> <image> [--broker=<broker>]

Options:
  -b --broker=<broker>  The broker URL.
  -h --help             Show this screen.
`

type command struct {
	// commands
	cCollect, cUpdate bool

	// arguments
	aBaseTopic, aImage string

	// options
	oBrokerURL string
}

func parseCommand() *command {
	a, _ := docopt.Parse(usage, nil, true, "", false)

	return &command{
		// commands
		cCollect: getBool(a["collect"]),
		cUpdate:  getBool(a["update"]),

		// arguments
		aBaseTopic: getString(a["<base-topic>"]),
		aImage:     getString(a["<image>"]),

		// options
		oBrokerURL: getString(a["--broker"]),
	}
}

func getBool(field interface{}) bool {
	if val, ok := field.(bool); ok {
		return val
	}

	return false
}

func getString(field interface{}) string {
	if str, ok := field.(string); ok {
		return str
	}

	return ""
}
