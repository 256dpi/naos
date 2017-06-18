package main

import (
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `naos - the networked artifacts operating system

Usage:
  naos create
  naos install [--force]
  naos build
  naos flash [--erase]
  naos attach
  naos list
  naos collect [--clear --duration=<ms>]
  naos get <param> [--filter=<pattern> --timeout=<ms>]
  naos set <param> <value> [--filter=<pattern> --timeout=<ms>]
  naos monitor [--filter=<pattern> --timeout=<ms>]
  naos record [--filter=<pattern>]
  naos update <image> [--filter=<pattern> --timeout=<ms>]

Options:
  -x --force             Force installation of components when they exist.
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
	oForce    bool
	oErase    bool
	oDuration time.Duration
	oTimeout  time.Duration
	oFilter   string
	oClear    bool
}

func parseCommand() *command {
	a, _ := docopt.Parse(usage, nil, true, "", false)

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
		oForce:    getBool(a["--force"]),
		oErase:    getBool(a["--erase"]),
		oDuration: getDuration(a["--duration"]),
		oTimeout:  getDuration(a["--timeout"]),
		oFilter:   getString(a["--filter"]),
		oClear:    getBool(a["--clear"]),
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
