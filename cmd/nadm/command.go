package main

import (
	"time"

	"github.com/docopt/docopt-go"
)

// TODO: Populate options from nadm.rc?

var usage = `nadm - the networked artifacts device manager

Usage:
  nadm collect [--broker=<url> --duration=<d>]
  nadm update <base-topic> <image> [--broker=<url>]

Options:
  -b --broker=<url>  The broker URL.
  -d --duration=<d>  The collection duration [default: 1s].
  -h --help          Show this screen.
`

type command struct {
	// commands
	cCollect, cUpdate bool

	// arguments
	aBaseTopic, aImage string

	// options
	oBrokerURL string
	oDuration  time.Duration
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
		oDuration:  getDuration(a["--duration"]),
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

func getDuration(field interface{}) time.Duration {
	d, _ := time.ParseDuration(getString(field))
	return d
}
