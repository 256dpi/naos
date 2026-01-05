package main

import (
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
)

var usage = `Networked Artifacts Operating System
© Joël Gähwiler
https://github.com/256dpi/naos

Fleet Management:
  create   Create a new fleet file.
  list     List all devices listed in the fleet.
  collect  Collect devices and add them to the fleet.
  discover Discover parameters and metrics of a device.
  ping     Ping devices.
  get      Read a parameter from devices.
  set      Set a parameter on devices.
  unset    Unset a parameter on devices.
  monitor  Monitor system information from devices.
  record   Record log messages from devices.
  debug    Gather coredump from devices.
  update   Update a firmware to devices.

Utilities:
  help     Show this help message.

Usage:
  naos-fleet create
  naos-fleet list
  naos-fleet collect [--clear --duration=<time>]
  naos-fleet discover [<pattern>] [--jobs=<count>]
  naos-fleet ping [<pattern>] [--jobs=<count>]
  naos-fleet get <param> [<pattern>] [--jobs=<count>]
  naos-fleet set <param> [--] <value> [<pattern>] [--jobs=<count>]
  naos-fleet unset <param> [<pattern>] [--jobs=<count>]
  naos-fleet monitor [<pattern>]
  naos-fleet record [<pattern>]
  naos-fleet debug [<pattern>] [--delete] [--jobs=<count>]
  naos-fleet update <version> <file> [<pattern>] [--jobs=<count>]
  naos-fleet help

Options:
  --clear               Remove not available devices from fleet.
  --delete              Delete loaded coredumps from the devices.
  -d --duration=<time>  Operation duration [default: 5s].
  -j --jobs=<count>     Number of simultaneous jobs [default: 10].
`

type command struct {
	// commands
	cCreate   bool
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
	cHelp     bool

	// arguments
	aDevice  string
	aParam   string
	aPattern string
	aValue   string
	aVersion string
	aFile    string

	// options
	oClear    bool
	oDelete   bool
	oDuration time.Duration
	oJobs     int
}

func parseCommand() *command {
	a, err := docopt.Parse(usage, nil, false, "", false)
	exitIfSet(err)

	return &command{
		// commands
		cCreate:   getBool(a["create"]),
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
		cHelp:     getBool(a["help"]),

		// arguments
		aDevice:  getString(a["<device>"]),
		aPattern: getString(a["<pattern>"]),
		aParam:   getString(a["<param>"]),
		aValue:   getString(a["<value>"]),
		aVersion: getString(a["<version>"]),
		aFile:    getString(a["<file>"]),

		// options
		oClear:    getBool(a["--clear"]),
		oDelete:   getBool(a["--delete"]),
		oDuration: getDuration(a["--duration"]),
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
