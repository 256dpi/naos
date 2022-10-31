package main

import (
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `Networked Artifacts Operating System
© Joël Gähwiler
https://github.com/256dpi/naos

Project Management:
  create   Create a new naos project in the current directory.
  install  Download required dependencies to the 'naos' subdirectory.
  build    Build all source files.
  flash    Flash the previously built binary to an attached device.
  attach   Open a serial communication with an attached device.
  run      Run 'build', 'flash' and 'attach' sequentially.
  exec     Run a command in the tree. 
  config   Write settings and parameters to an attached device.
  format   Format all source files in the 'src' subdirectory.

Fleet Management:
  list     List all devices listed in the inventory.
  collect  Collect devices and add them to the inventory.
  ping     Ping devices.
  send     Send a message to devices.
  discover Discover all parameters of a device.
  get      Read a parameter from devices.
  set      Set a parameter on devices.
  unset    Unset a parameter on devices.
  monitor  Monitor heartbeats from devices.
  record   Record log messages from devices.
  debug    Gather debug information from devices.
  update   Update devices over the air.

Usage:
  naos create [--cmake --force]
  naos install [--force]
  naos build [--clean --app-only]
  naos flash [<device>] [--baud=<rate> --erase --app-only]
  naos attach [<device>] [--simple]
  naos run [<device>] [--clean --app-only --baud=<rate> --erase --simple]
  naos exec <command>
  naos config <file> [<device>]
  naos format
  naos list
  naos collect [--clear --duration=<time>]
  naos ping [<pattern>] [--timeout=<time>]
  naos send <topic> [--] <message> [<pattern>] [--timeout=<time>]
  naos discover [<pattern>] [--timeout=<time>]
  naos get <param> [<pattern>] [--timeout=<time>]
  naos set <param> [--] <value> [<pattern>] [--timeout=<time>]
  naos unset <param> [<pattern>] [--timeout=<time>]
  naos monitor [<pattern>] [--timeout=<time>]
  naos record [<pattern>] [--timeout=<time>]
  naos debug [<pattern>] [--delete --duration=<time>]
  naos update <version> [<pattern>] [--jobs=<count> --timeout=<time>]
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
  -b --baud=<rate>      The baud rate.
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
	cExec     bool
	cConfig   bool
	cFormat   bool
	cList     bool
	cCollect  bool
	cPing     bool
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
	aFile    string
	aCommand string
	aParam   string
	aPattern string
	aTopic   string
	aMessage string
	aValue   string
	aVersion string

	// options
	oForce    bool
	oBaudRate string
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
		cExec:     getBool(a["exec"]),
		cConfig:   getBool(a["config"]),
		cFormat:   getBool(a["format"]),
		cList:     getBool(a["list"]),
		cCollect:  getBool(a["collect"]),
		cPing:     getBool(a["ping"]),
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
		aFile:    getString(a["<file>"]),
		aCommand: getString(a["<command>"]),
		aPattern: getString(a["<pattern>"]),
		aTopic:   getString(a["<topic>"]),
		aMessage: getString(a["<message>"]),
		aParam:   getString(a["<param>"]),
		aValue:   getString(a["<value>"]),
		aVersion: getString(a["<version>"]),

		// options
		oForce:    getBool(a["--force"]),
		oCMake:    getBool(a["--cmake"]),
		oClean:    getBool(a["--clean"]),
		oBaudRate: getString(a["--baud"]),
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
