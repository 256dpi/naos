package main

import (
	"path/filepath"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `nadm - the networked artifacts device manager

Usage:
  nadm create [--inventory=<file>]
  nadm list [--inventory=<file>]
  nadm collect [--clear --duration=<ms> --inventory=<file>]
  nadm get <param> [--filter=<pattern> --timeout=<ms> --inventory=<file>]
  nadm set <param> <value> [--filter=<pattern> --timeout=<ms> --inventory=<file>]
  nadm monitor [--filter=<pattern> --timeout=<ms> --inventory=<file>]
  nadm update <image> [--filter=<pattern> --timeout=<ms> --inventory=<file>]

Options:
  -i --inventory=<file>  The inventory file [default: ./nadm.json].
  -d --duration=<ms>     The collection duration [default: 1s].
  -t --timeout=<ms>      The response timeout [default: 5s].
  -f --filter=<pattern>  The filter glob pattern [default: *].
  -c --clear             Remove not available devices from inventory.
  -h --help              Show this screen.
`

type command struct {
	// commands
	cCreate  bool
	cList    bool
	cCollect bool
	cGet     bool
	cSet     bool
	cMonitor bool
	cUpdate  bool

	// arguments
	aParam string
	aValue string
	aName  string
	aImage string

	// options
	oInventory string
	oDuration  time.Duration
	oTimeout   time.Duration
	oFilter    string
	oClear     bool
}

func parseCommand() *command {
	a, _ := docopt.Parse(usage, nil, true, "", false)

	inv := getString(a["--inventory"])
	if inv == "./nadm.json" {
		i, err := filepath.Abs("nadm.json")
		exitIfSet(err)
		inv = i
	} else {
		i, err := filepath.Abs(inv)
		exitIfSet(err)
		inv = i
	}

	return &command{
		// commands
		cCreate:  getBool(a["create"]),
		cList:    getBool(a["list"]),
		cCollect: getBool(a["collect"]),
		cMonitor: getBool(a["monitor"]),
		cUpdate:  getBool(a["update"]),
		cSet:     getBool(a["set"]),
		cGet:     getBool(a["get"]),

		// arguments
		aName:  getString(a["<name>"]),
		aImage: getString(a["<image>"]),

		aParam: getString(a["<param>"]),
		aValue: getString(a["<value>"]),

		// options
		oInventory: inv,
		oDuration:  getDuration(a["--duration"]),
		oTimeout:   getDuration(a["--timeout"]),
		oFilter:    getString(a["--filter"]),
		oClear:     getBool(a["--clear"]),
	}
}

func getBool(field interface{}) bool {
	val, _ := field.(bool)
	return val
}

func getString(field interface{}) string {
	str, _ := field.(string)
	return str
}

func getDuration(field interface{}) time.Duration {
	d, _ := time.ParseDuration(getString(field))
	return d
}
