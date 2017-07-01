package main

import (
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `Networked Artifacts Operating System by shiftr.io

Project Management:
  create  Will create a new naos project in the current directory.
  setup   Will download required dependencies to the 'naos' subdirectory.
  build   Will build all source files.
  flash   Will flash the previously built binary to an attached device.
  attach  Will open a serial communication with an attached device.
  format  Will format all source files in the 'src' subdirectory.

Fleet Management:
  list     Will list all devices listed in the inventory.
  collect  Will collect connected devices and add them to the inventory.
  get      Will read a parameter value from connected devices.
  set      Will set a parameter value on connected devices.
  monitor  Will monitor heartbeats from connected devices.
  record   Will record log messages from connected devices.
  update   Will send the previously built binary to connected devices.

Usage:
  naos create
  naos setup [--force --cmake]
  naos build [--clean --app-only]
  naos flash [<device>] [--erase --app-only]
  naos attach [<device>] [--simple]
  naos format
  naos list
  naos collect [--clear --duration=<ms>]
  naos get <param> [<pattern>] [--timeout=<ms>]
  naos set <param> <value> [<pattern>] [--timeout=<ms>]
  naos monitor [<pattern>] [--timeout=<ms>]
  naos record [<pattern>]
  naos update [<pattern>] [--timeout=<ms>]
  naos help

Options:
  --force          Reinstall dependencies when they already exist.
  --cmake          Create required CMake files for IDEs like CLion.
  --clean          Clean all build artifacts before building again.
  --erase          Erase completely before flashing new image.
  --app-only       Only build or flash the application.
  --simple         Use simple serial tool.
  --clear          Remove not available devices from inventory.
  --duration=<ms>  Collection duration [default: 1s].
  --timeout=<ms>   Response timeout [default: 5s].
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
	cHelp    bool

	// arguments
	aDevice  string
	aParam   string
	aPattern string
	aValue   string
	aName    string

	// options
	oForce    bool
	oCMake    bool
	oClean    bool
	oErase    bool
	oAppOnly  bool
	oSimple   bool
	oClear    bool
	oDuration time.Duration
	oTimeout  time.Duration
}

func parseCommand() *command {
	a, err := docopt.Parse(usage, nil, false, "", false)
	exitIfSet(err)

	return &command{
		// commands
		cCreate:  getBool(a["create"]),
		cSetup:   getBool(a["setup"]),
		cBuild:   getBool(a["build"]),
		cFlash:   getBool(a["flash"]),
		cAttach:  getBool(a["attach"]),
		cFormat:  getBool(a["format"]),
		cList:    getBool(a["list"]),
		cCollect: getBool(a["collect"]),
		cSet:     getBool(a["set"]),
		cGet:     getBool(a["get"]),
		cMonitor: getBool(a["monitor"]),
		cRecord:  getBool(a["record"]),
		cUpdate:  getBool(a["update"]),
		cHelp:    getBool(a["help"]),

		// arguments
		aDevice:  getString(a["<device>"]),
		aName:    getString(a["<name>"]),
		aPattern: getString(a["<pattern>"]),
		aParam:   getString(a["<param>"]),
		aValue:   getString(a["<value>"]),

		// options
		oForce:    getBool(a["--force"]),
		oCMake:    getBool(a["--cmake"]),
		oClean:    getBool(a["--clean"]),
		oErase:    getBool(a["--erase"]),
		oAppOnly:  getBool(a["--app-only"]),
		oSimple:   getBool(a["--simple"]),
		oClear:    getBool(a["--clear"]),
		oDuration: getDuration(a["--duration"]),
		oTimeout:  getDuration(a["--timeout"]),
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
