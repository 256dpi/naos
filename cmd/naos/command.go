package main

import (
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `naos - the networked artifacts operating system

Usage:
  naos create
  naos setup [--force --verbose]
  naos build [--verbose --app-only]
  naos flash [--erase --app-only --verbose]
  naos attach
  naos list
  naos collect [--clear --duration=<ms>]
  naos get <param> [<pattern>] [--timeout=<ms>]
  naos set <param> <value> [<pattern>] [--timeout=<ms>]
  naos monitor [<pattern>] [--timeout=<ms>]
  naos record [<pattern>]
  naos update <image> [<pattern>] [--timeout=<ms>]

Options:
  -f --force          Force a re-installation of components when they exist.
  -e --erase          Erase completely before flashing new image.
  -a --app-only       If set only the app will be built or flashed.
  -d --duration=<ms>  The collection duration [default: 1s].
  -t --timeout=<ms>   The response timeout [default: 5s].
  -c --clear          Remove not available devices from inventory.
  -v --verbose        Be verbose about whats going on.
  -h --help           Show this screen.
`

type command struct {
	// commands
	cCreate  bool
	cSetup   bool
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
	aParam   string
	aPattern string
	aValue   string
	aName    string
	aImage   string

	// options
	oForce    bool
	oErase    bool
	oAppOnly  bool
	oDuration time.Duration
	oTimeout  time.Duration
	oClear    bool
	oVerbose  bool
}

func parseCommand() *command {
	a, err := docopt.Parse(usage, nil, true, "", false)
	exitIfSet(err)

	return &command{
		// commands
		cCreate:  getBool(a["create"]),
		cSetup:   getBool(a["setup"]),
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
		aName:    getString(a["<name>"]),
		aPattern: getString(a["<pattern>"]),
		aImage:   getString(a["<image>"]),
		aParam:   getString(a["<param>"]),
		aValue:   getString(a["<value>"]),

		// options
		oForce:    getBool(a["--force"]),
		oErase:    getBool(a["--erase"]),
		oAppOnly:  getBool(a["--app-only"]),
		oDuration: getDuration(a["--duration"]),
		oTimeout:  getDuration(a["--timeout"]),
		oClear:    getBool(a["--clear"]),
		oVerbose:  getBool(a["--verbose"]),
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
