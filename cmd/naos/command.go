package main

import (
	"time"

	"github.com/docopt/docopt-go"
)

// TODO: Support specifying device file for flash and attach.

var usage = `naos - the networked artifacts operating system

Usage:
  naos create
  naos setup [--force --cmake]
  naos build [--app-only]
  naos flash [--erase --app-only]
  naos attach
  naos fmt
  naos list
  naos collect [--clear --duration=<ms>]
  naos get <param> [<pattern>] [--timeout=<ms>]
  naos set <param> <value> [<pattern>] [--timeout=<ms>]
  naos monitor [<pattern>] [--timeout=<ms>]
  naos record [<pattern>]
  naos update [<pattern>] [--image=<path> --timeout=<ms>]

Options:
  -f --force          Force a re-installation of components when they exist.
  -m --cmake          Will create required CMake files for IDEs like CLion.
  -e --erase          Erase completely before flashing new image.
  -a --app-only       If set only the app will be built or flashed.
  -d --duration=<ms>  The collection duration [default: 1s].
  -t --timeout=<ms>   The response timeout [default: 5s].
  -i --image=<path>   Path to the binary app image.
  -c --clear          Remove not available devices from inventory.
  -h --help           Show this screen.
`

type command struct {
	// commands
	cCreate  bool
	cSetup   bool
	cBuild   bool
	cFlash   bool
	cAttach  bool
	cFormat  bool
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

	// options
	oForce    bool
	oCMake    bool
	oErase    bool
	oClear    bool
	oAppOnly  bool
	oDuration time.Duration
	oTimeout  time.Duration
	oImage    string
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
		cFormat:  getBool(a["fmt"]),
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
		aParam:   getString(a["<param>"]),
		aValue:   getString(a["<value>"]),

		// options
		oForce:    getBool(a["--force"]),
		oCMake:    getBool(a["--cmake"]),
		oErase:    getBool(a["--erase"]),
		oClear:    getBool(a["--clear"]),
		oAppOnly:  getBool(a["--app-only"]),
		oDuration: getDuration(a["--duration"]),
		oTimeout:  getDuration(a["--timeout"]),
		oImage:    getString(a["--image"]),
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
