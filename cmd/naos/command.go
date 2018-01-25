package main

import (
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `Networked Artifacts Operating System
Created by Joël Gähwiler © shiftr.io

Project Management:
  create   Will create a new naos project in the current directory.
  install  Will download required dependencies to the 'naos' subdirectory.
  build    Will build all source files.
  flash    Will flash the previously built binary to an attached device.
  attach   Will open a serial communication with an attached device.
  run      Will run 'build', 'flash' and 'attach' sequentially.
  format   Will format all source files in the 'src' subdirectory.

Configuration Management:
  scan     Will scan for bluetooth devices and print their configuration.
  rename   Will reset the device name of the device with the specified address.

Fleet Management:
  list     Will list all devices listed in the inventory.
  collect  Will collect devices and add them to the inventory.
  get      Will read a parameter value from devices.
  set      Will set a parameter value on devices.
  unset    Will unset a parameter on devices.
  monitor  Will monitor heartbeats from devices.
  record   Will record log messages from devices.
  debug    Will gather debug information from devices.
  update   Will send the previously built binary to devices.

Usage:
  naos create [--cmake --force]
  naos install [--force]
  naos build [--clean --app-only]
  naos flash [<device>] [--erase --app-only]
  naos attach [<device>] [--simple]
  naos run [<device>] [--clean --app-only --erase --simple]
  naos format
  naos scan [--duration=<time>]
  naos rename <address> <name>
  naos list
  naos collect [--clear --duration=<time>]
  naos get <param> [<pattern>] [--timeout=<time>]
  naos set <param> <value> [<pattern>] [--timeout=<time>]
  naos unset <param> [<pattern>] [--timeout=<time>]
  naos monitor [<pattern>] [--timeout=<time>]
  naos record [<pattern>] [--timeout=<time>]
  naos debug [<pattern>] [--delete --duration=<time>]
  naos update [<pattern>] [--jobs=<count> --timeout=<time>]
  naos help

Options:
  --cmake               Create required CMake files for IDEs like CLion.
  --force               Reinstall dependencies when they already exist.
  --clean               Clean all build artifacts before building again.
  --erase               Erase completely before flashing new image.
  --app-only            Only build or flash the application.
  --simple              Use simple serial tool.
  --clear               Remove not available devices from inventory.
  --delete              Delete loaded coredumps from the devices.
  -d --duration=<time>  Operation duration [default: 2s].
  -t --timeout=<time>   Operation timeout [default: 5s].
  -j --jobs=<count>     Number of simultaneous update jobs [default: 10].
`

type command struct {
	// commands
	cCreate  bool
	cInstall bool
	cBuild   bool
	cFlash   bool
	cAttach  bool
	cRun     bool
	cFormat  bool
	cList    bool
	cScan    bool
	cRename  bool
	cCollect bool
	cGet     bool
	cSet     bool
	cUnset   bool
	cMonitor bool
	cRecord  bool
	cDebug   bool
	cUpdate  bool
	cHelp    bool

	// arguments
	aDevice  string
	aAddress string
	aName    string
	aParam   string
	aPattern string
	aValue   string

	// options
	oForce    bool
	oCMake    bool
	oClean    bool
	oErase    bool
	oAppOnly  bool
	oSimple   bool
	oClear    bool
	oDelete   bool
	oDuration time.Duration
	oTimeout  time.Duration
	oJobs     int
}

func parseCommand() *command {
	a, err := docopt.Parse(usage, nil, false, "", false)
	exitIfSet(err)

	return &command{
		// commands
		cCreate:  getBool(a["create"]),
		cInstall: getBool(a["install"]),
		cBuild:   getBool(a["build"]),
		cFlash:   getBool(a["flash"]),
		cAttach:  getBool(a["attach"]),
		cRun:     getBool(a["run"]),
		cFormat:  getBool(a["format"]),
		cList:    getBool(a["list"]),
		cScan:    getBool(a["scan"]),
		cRename:  getBool(a["rename"]),
		cCollect: getBool(a["collect"]),
		cGet:     getBool(a["get"]),
		cSet:     getBool(a["set"]),
		cUnset:   getBool(a["unset"]),
		cMonitor: getBool(a["monitor"]),
		cRecord:  getBool(a["record"]),
		cDebug:   getBool(a["debug"]),
		cUpdate:  getBool(a["update"]),
		cHelp:    getBool(a["help"]),

		// arguments
		aDevice:  getString(a["<device>"]),
		aAddress: getString(a["<address>"]),
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
		oDelete:   getBool(a["--delete"]),
		oDuration: getDuration(a["--duration"]),
		oTimeout:  getDuration(a["--timeout"]),
		oJobs:     getInt(a["--jobs"]),
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

func getInt(field interface{}) int {
	num, _ := strconv.Atoi(getString(field))
	return num
}

func getDuration(field interface{}) time.Duration {
	d, _ := time.ParseDuration(getString(field))
	return d
}
