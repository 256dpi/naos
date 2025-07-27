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
  run      Run 'build', 'flash' and 'attach' sequentially.
  trace    Run 'build' and 'flash' with app tracing enabled over USB.
  exec     Run a command in the tree. 
  config   Write parameters to an attached device or to devices over BLE.
  format   Format all source files in the 'src' subdirectory.
  bundle   Generate a bundle of the project.

Fleet Management:
  list     List all devices listed in the inventory.
  collect  Collect devices and add them to the inventory.
  ping     Ping devices.
  send     Send a message to devices.
  receive  Receive messages from devices.
  discover Discover all parameters of a device.
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
  naos trace [<device>] [--cpu=<core> --clean --reconfigure --app-only --baud=<rate> --erase --alt]
  naos exec <command>
  naos config <file> [<device>] [--baud=<rate> --ble]
  naos format
  naos bundle [<file>]
  naos list
  naos collect [--clear --duration=<time>]
  naos ping [<pattern>] [--timeout=<time>]
  naos send <topic> [--] <message> [<pattern>] [--timeout=<time>]
  naos receive <topic> [--] [<pattern>] [--timeout=<time>]
  naos discover [<pattern>] [--timeout=<time>]
  naos get <param> [<pattern>] [--timeout=<time>]
  naos set <param> [--] <value> [<pattern>] [--timeout=<time>]
  naos unset <param> [<pattern>] [--timeout=<time>]
  naos monitor [<pattern>] [--timeout=<time>]
  naos record [<pattern>] [--timeout=<time>]
  naos debug [<pattern>] [--delete --duration=<time>]
  naos update <version> [<pattern>] [--jobs=<count> --timeout=<time>]
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
  --ble                 Use BLE instead of serial to configure devices.
  --clear               Remove not available devices from inventory.
  --delete              Delete loaded coredumps from the devices.
  -b --baud=<rate>      The baud rate.
  -c --cpu=<core>       The CPU core to trace (0: Protocol, 1: Application) [default: 1].
  -d --duration=<time>  Operation duration [default: 5s].
  -t --timeout=<time>   Operation timeout [default: 30s].
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
	cTrace    bool
	cExec     bool
	cConfig   bool
	cFormat   bool
	cBundle   bool
	cList     bool
	cCollect  bool
	cPing     bool
	cSend     bool
	cReceive  bool
	cDiscover bool
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
	oCPUCore     string
	oBLE         bool
	oClear       bool
	oDelete      bool
	oDuration    time.Duration
	oTimeout     time.Duration
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
		cTrace:    getBool(a["trace"]),
		cExec:     getBool(a["exec"]),
		cConfig:   getBool(a["config"]),
		cFormat:   getBool(a["format"]),
		cBundle:   getBool(a["bundle"]),
		cList:     getBool(a["list"]),
		cCollect:  getBool(a["collect"]),
		cPing:     getBool(a["ping"]),
		cSend:     getBool(a["send"]),
		cReceive:  getBool(a["receive"]),
		cDiscover: getBool(a["discover"]),
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
		oCPUCore:     getString(a["--cpu"]),
		oBLE:         getBool(a["--ble"]),
		oClear:       getBool(a["--clear"]),
		oDelete:      getBool(a["--delete"]),
		oDuration:    getDuration(a["--duration"]),
		oTimeout:     getDuration(a["--timeout"]),
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
