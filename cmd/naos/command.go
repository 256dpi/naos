package main

import (
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
  naos sdks
  naos help

Options:
  --cmake            Create required CMake files for IDEs like CLion.
  --force            Reinstall dependencies when they already exist.
  --clean            Clean all build artifacts before building again.
  --reconfigure      Reconfigure will recalculate the sdkconfig file.
  --erase            Erase completely before flashing new image.
  --app-only         Only build or flash the application.
  --alt              Use alternative esptool.py found in PATH.
  -b --baud=<rate>   The baud rate.
`

type command struct {
	// commands
	cCreate  bool
	cInstall bool
	cBuild   bool
	cDetect  bool
	cFlash   bool
	cAttach  bool
	cRun     bool
	cExec    bool
	cConfig  bool
	cFormat  bool
	cBundle  bool
	cSDKs    bool
	cHelp    bool

	// arguments
	aDevice  string
	aFile    string
	aCommand string

	// options
	oForce       bool
	oBaudRate    string
	oCMake       bool
	oClean       bool
	oReconfigure bool
	oErase       bool
	oAppOnly     bool
	oAlt         bool
}

func parseCommand() *command {
	a, err := docopt.Parse(usage, nil, false, "", false)
	exitIfSet(err)

	return &command{
		// commands
		cCreate:  getBool(a["create"]),
		cInstall: getBool(a["install"]),
		cBuild:   getBool(a["build"]),
		cDetect:  getBool(a["detect"]),
		cFlash:   getBool(a["flash"]),
		cAttach:  getBool(a["attach"]),
		cRun:     getBool(a["run"]),
		cExec:    getBool(a["exec"]),
		cConfig:  getBool(a["config"]),
		cFormat:  getBool(a["format"]),
		cBundle:  getBool(a["bundle"]),
		cSDKs:    getBool(a["sdks"]),
		cHelp:    getBool(a["help"]),

		// arguments
		aDevice:  getString(a["<device>"]),
		aFile:    getString(a["<file>"]),
		aCommand: getString(a["<command>"]),

		// options
		oForce:       getBool(a["--force"]),
		oCMake:       getBool(a["--cmake"]),
		oClean:       getBool(a["--clean"]),
		oReconfigure: getBool(a["--reconfigure"]),
		oBaudRate:    getString(a["--baud"]),
		oErase:       getBool(a["--erase"]),
		oAppOnly:     getBool(a["--app-only"]),
		oAlt:         getBool(a["--alt"]),
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
