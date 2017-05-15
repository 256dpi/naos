package main

import "github.com/docopt/docopt-go"

// TODO: Url as option and populate from nadm.rc.

var usage = `nadm - the networked artifacts device manager

Usage:
  nadm update <url> <base-topic> <image>

Options:
  -h --help  Show this screen.
`

type command struct {
	// commands
	cUpdate bool

	// arguments
	aURL, aBaseTopic, aImage string
}

func parseCommand() *command {
	a, _ := docopt.Parse(usage, nil, true, "", false)

	return &command{
		// commands
		cUpdate: getBool(a["update"]),

		// arguments
		aURL:       getString(a["<url>"]),
		aBaseTopic: getString(a["<base-topic>"]),
		aImage:     getString(a["<image>"]),
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
