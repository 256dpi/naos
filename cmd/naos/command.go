package main

import (
	"path/filepath"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `naos - the networked artifacts operating system

Usage:
  naos create [--inventory=<file>]
  naos install [--inventory=<file>]
  naos build [--inventory=<file>]
  naos flash [--erase --inventory=<file>]
  naos attach
  naos list [--inventory=<file>]
  naos collect [--clear --duration=<ms> --inventory=<file>]
  naos get <param> [--filter=<pattern> --timeout=<ms> --inventory=<file>]
  naos set <param> <value> [--filter=<pattern> --timeout=<ms> --inventory=<file>]
  naos monitor [--filter=<pattern> --timeout=<ms> --inventory=<file>]
  naos record [--filter=<pattern> --inventory=<file>]
  naos update <image> [--filter=<pattern> --timeout=<ms> --inventory=<file>]

Options:
  -i --inventory=<file>  The inventory file [default: ./naos.json].
  -e --erase             Erase completely before flashing new image.
  -d --duration=<ms>     The collection duration [default: 1s].
  -t --timeout=<ms>      The response timeout [default: 5s].
  -f --filter=<pattern>  The filter glob pattern [default: *].
  -c --clear             Remove not available devices from inventory.
  -h --help              Show this screen.
`

type command struct {
	// commands
	cCreate  bool
	cInstall bool
	cBuild   bool
	cFlash   bool
	cAttach  bool
	cList    bool
	cCollect bool
	cGet     bool
	cSet     bool
	cMonitor bool
	cRecord  bool
	cUpdate  bool

	// arguments
	aParam string
	aValue string
	aName  string
	aImage string

	// options
	oInventory string
	oErase     bool
	oDuration  time.Duration
	oTimeout   time.Duration
	oFilter    string
	oClear     bool
}

func parseCommand() *command {
	a, _ := docopt.Parse(usage, nil, true, "", false)

	inv := getString(a["--inventory"])
	if inv == "./naos.json" {
		i, err := filepath.Abs("naos.json")
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
		cInstall: getBool(a["install"]),
		cBuild:   getBool(a["build"]),
		cFlash:   getBool(a["flash"]),
		cAttach:  getBool(a["attach"]),
		cList:    getBool(a["list"]),
		cCollect: getBool(a["collect"]),
		cSet:     getBool(a["set"]),
		cGet:     getBool(a["get"]),
		cMonitor: getBool(a["monitor"]),
		cRecord:  getBool(a["record"]),
		cUpdate:  getBool(a["update"]),

		// arguments
		aName:  getString(a["<name>"]),
		aImage: getString(a["<image>"]),

		aParam: getString(a["<param>"]),
		aValue: getString(a["<value>"]),

		// options
		oInventory: inv,
		oErase:     getBool(a["--erase"]),
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
