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
  detect   Detect all connected devices.
  flash    Flash the previously built binary to an attached device.
  attach   Open a serial communication with an attached device.
  run      Run 'build', 'flash', and 'attach' sequentially.
  exec     Run a command in the tree. 
  config   Write parameters to an attached device.
  format   Format all source files in the 'src' subdirectory.
  bundle   Generate a bundle of the project.

Fleet Management:
  list     List all devices listed in the inventory.
  collect  Collect devices and add them to the inventory.
  discover Discover all parameters of a device.
  ping     Ping devices.
  get      Read a parameter from devices.
  set      Set a parameter on devices.
  unset    Unset a parameter on devices.
  monitor  Monitor heartbeats from devices.
  record   Record log messages from devices.
  debug    Gather debug information from devices.
  update   Update devices over the air.

Utilities:
  sdks     List installed SDKs with their versions and location.
  help     Show this help message.

Usage:
  naos create [--cmake --force]
  naos install [--force]
  naos build [--clean --reconfigure --app-only]
  naos detect
  naos flash [<device>] [--baud=<rate> --erase --app-only --alt]
  naos attach [<device>]
  naos run [<device>] [--clean --reconfigure --app-only --baud=<rate> --erase --alt]
  naos exec <command>
  naos config <file> [<device>] [--baud=<rate>]
  naos format
  naos bundle [<file>]
  naos list
  naos collect [--clear --duration=<time>]
  naos discover [<pattern>] [--jobs=<count>]
  naos ping [<pattern>] [--jobs=<count>]
  naos get <param> [<pattern>] [--jobs=<count>]
  naos set <param> [--] <value> [<pattern>] [--jobs=<count>]
  naos unset <param> [<pattern>] [--jobs=<count>]
  naos monitor [<pattern>]
  naos record [<pattern>]
  naos debug [<pattern>] [--delete] [--jobs=<count>]
  naos update <version> [<pattern>] [--jobs=<count>]
  naos sdks
  naos help

Options:
  --cmake               Create required CMake files for IDEs like CLion.
  --force               Reinstall dependencies when they already exist.
  --clean               Clean all build artifacts before building again.
  --reconfigure         Reconfigure will recalculate the sdkconfig file.
  --erase               Erase completely before flashing new image.
  --app-only            Only build or flash the application.
  --alt                 Use alternative esptool.py found in PATH.
  --clear               Remove not available devices from inventory.
  --delete              Delete loaded coredumps from the devices.
  -b --baud=<rate>      The baud rate.
  -d --duration=<time>  Operation duration [default: 5s].
  -j --jobs=<count>     Number of simultaneous update jobs [default: 10].
`

type command struct {
	// commands
	cCreate   bool
	cInstall  bool
	cBuild    bool
	cDetect   bool
	cFlash    bool
	cAttach   bool
	cRun      bool
	cExec     bool
	cConfig   bool
	cFormat   bool
	cBundle   bool
	cList     bool
	cCollect  bool
	cDiscover bool
	cPing     bool
	cGet      bool
	cSet      bool
	cUnset    bool
	cMonitor  bool
	cRecord   bool
	cDebug    bool
	cUpdate   bool
	cSDKs     bool
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
	oForce       bool
	oBaudRate    string
	oCMake       bool
	oClean       bool
	oReconfigure bool
	oErase       bool
	oAppOnly     bool
	oAlt         bool
	oClear       bool
	oDelete      bool
	oDuration    time.Duration
	oJobs        int
}

func parseCommand() *command {
	a, err := docopt.Parse(usage, nil, false, "", false)
	exitIfSet(err)

	return &command{
		// commands
		cCreate:   getBool(a["create"]),
		cInstall:  getBool(a["install"]),
		cBuild:    getBool(a["build"]),
		cDetect:   getBool(a["detect"]),
		cFlash:    getBool(a["flash"]),
		cAttach:   getBool(a["attach"]),
		cRun:      getBool(a["run"]),
		cExec:     getBool(a["exec"]),
		cConfig:   getBool(a["config"]),
		cFormat:   getBool(a["format"]),
		cBundle:   getBool(a["bundle"]),
		cList:     getBool(a["list"]),
		cCollect:  getBool(a["collect"]),
		cDiscover: getBool(a["discover"]),
		cPing:     getBool(a["ping"]),
		cGet:      getBool(a["get"]),
		cSet:      getBool(a["set"]),
		cUnset:    getBool(a["unset"]),
		cMonitor:  getBool(a["monitor"]),
		cRecord:   getBool(a["record"]),
		cDebug:    getBool(a["debug"]),
		cUpdate:   getBool(a["update"]),
		cSDKs:     getBool(a["sdks"]),
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
		oForce:       getBool(a["--force"]),
		oCMake:       getBool(a["--cmake"]),
		oClean:       getBool(a["--clean"]),
		oReconfigure: getBool(a["--reconfigure"]),
		oBaudRate:    getString(a["--baud"]),
		oErase:       getBool(a["--erase"]),
		oAppOnly:     getBool(a["--app-only"]),
		oAlt:         getBool(a["--alt"]),
		oClear:       getBool(a["--clear"]),
		oDelete:      getBool(a["--delete"]),
		oDuration:    getDuration(a["--duration"]),
		oJobs:        getInt(a["--jobs"]),
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
