package main

import (
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `Networked Artifacts Operating System
Created by Joël Gähwiler © shiftr.io
https://github.com/shiftr-io/naos

Project Management:
  create   Create a new naos project in the current directory.
  install  Download required dependencies to the 'naos' subdirectory.
  build    Build all source files.
  flash    Flash the previously built binary to an attached device.
  attach   Open a serial communication with an attached device.
  run      Run 'build', 'flash' and 'attach' sequentially.
  format   Format all source files in the 'src' subdirectory.

Fleet Management:
  list     List all devices listed in the inventory.
  collect  Collect devices and add them to the inventory.
  send     Send a message to devices.
  discover Discover all parameters of a device.
  get      Read a parameter from devices.
  set      Set a parameter on devices.
  unset    Unset a parameter on devices.
  monitor  Monitor heartbeats from devices.
  record   Record log messages from devices.
  debug    Gather debug information from devices.
  update   Send the previously built binary to devices.

Usage:
  naos create [--cmake --force]
  naos install [--force]
  naos build [--clean --app-only]
  naos flash [<device>] [--erase --app-only]
  naos attach [<device>] [--simple]
  naos run [<device>] [--clean --app-only --erase --simple]
  naos format
  naos list
  naos collect [--clear --duration=<time>]
  naos send <topic> [--] <message> [<pattern>] [--timeout=<time>]
  naos discover [pattern] [--timeout=<time>]
  naos get <param> [<pattern>] [--timeout=<time>]
  naos set <param> [--] <value> [<pattern>] [--timeout=<time>]
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
	cCreate   bool
	cInstall  bool
	cBuild    bool
	cFlash    bool
	cAttach   bool
	cRun      bool
	cFormat   bool
	cList     bool
	cCollect  bool
	cSend     bool
	cDiscover bool
	cGet      bool
	cSet      bool
	cUnset    bool
	cMonitor  bool
	cRecord   bool
	cDebug    bool
	cUpdate   bool
	cHelp     bool

	// arguments
	aDevice  string
	aParam   string
	aPattern string
	aTopic   string
	aMessage string
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
		cCreate:   getBool(a["create"]),
		cInstall:  getBool(a["install"]),
		cBuild:    getBool(a["build"]),
		cFlash:    getBool(a["flash"]),
		cAttach:   getBool(a["attach"]),
		cRun:      getBool(a["run"]),
		cFormat:   getBool(a["format"]),
		cList:     getBool(a["list"]),
		cCollect:  getBool(a["collect"]),
		cSend:     getBool(a["send"]),
		cDiscover: getBool(a["discover"]),
		cGet:      getBool(a["get"]),
		cSet:      getBool(a["set"]),
		cUnset:    getBool(a["unset"]),
		cMonitor:  getBool(a["monitor"]),
		cRecord:   getBool(a["record"]),
		cDebug:    getBool(a["debug"]),
		cUpdate:   getBool(a["update"]),
		cHelp:     getBool(a["help"]),

		// arguments
		aDevice:  getString(a["<device>"]),
		aPattern: getString(a["<pattern>"]),
		aTopic:   getString(a["<topic>"]),
		aMessage: getString(a["<message>"]),
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
