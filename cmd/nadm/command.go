package main

import (
	"path/filepath"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `nadm - the networked artifacts device manager

Usage:
  nadm create [--inventory=<file>]
  nadm collect [--duration=<d> --clear --inventory=<file>]
  nadm set <filter> <param> <value> [--inventory=<file>]
  nadm get <filter> <param> [--inventory=<file>]
  nadm monitor <filter> [--inventory=<file>]
  nadm update <name> <image> [--inventory=<file>]

Options:
  -i --inventory=<file>  The inventory file [default: ./nadm.json].
  -d --duration=<d>      The collection duration [default: 1s].
  -c --clear             Remove not available devices from inventory.
  -h --help              Show this screen.
`

type command struct {
	// commands
	cCreate, cCollect, cMonitor, cUpdate, cSet, cGet bool

	// arguments
	aName, aImage, aFilter, aParam, aValue string

	// options
	oInventory string
	oClear     bool
	oDuration  time.Duration
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
		cCollect: getBool(a["collect"]),
		cMonitor: getBool(a["monitor"]),
		cUpdate:  getBool(a["update"]),
		cSet:     getBool(a["set"]),
		cGet:     getBool(a["get"]),

		// arguments
		aName:   getString(a["<name>"]),
		aImage:  getString(a["<image>"]),
		aFilter: getString(a["<filter>"]),
		aParam:  getString(a["<param>"]),
		aValue:  getString(a["<value>"]),

		// options
		oInventory: inv,
		oClear:     getBool(a["--clear"]),
		oDuration:  getDuration(a["--duration"]),
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
