package main

import (
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/naos"
	"github.com/256dpi/naos/pkg/sdk"
	"github.com/256dpi/naos/pkg/serial"
)

func main() {
	// parse command
	cmd, err := parseCommand(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	// run desired command
	switch cmd.name {
	case "create":
		create(cmd)
	case "install":
		install(cmd, getProject())
	case "build":
		build(cmd, getProject())
	case "detect":
		detect()
	case "flash":
		flash(cmd, getProject())
	case "attach":
		attach(cmd, getProject())
	case "run":
		run(cmd, getProject())
	case "exec":
		exec(cmd, getProject())
	case "config list":
		configList(cmd)
	case "config get":
		configGet(cmd)
	case "config set":
		configSet(cmd)
	case "config unset":
		configUnset(cmd)
	case "config read":
		configRead(cmd)
	case "config write":
		configWrite(cmd)
	case "config flash":
		configFlash(cmd, getProject())
	case "format":
		format(cmd, getProject())
	case "bundle":
		bundle(cmd, getProject())
	case "debug":
		debug(cmd, getProject())
	case "sdks":
		sdks(cmd)
	}
}

func create(cmd *command) {
	// create project
	p, err := naos.CreateProject(workingDirectory(), cmd.oForce, cmd.oCMake, os.Stdout)
	exitIfSet(err)

	// save manifest
	exitIfSet(p.SaveManifest())
}

func install(cmd *command, p *naos.Project) {
	// install dependencies
	exitIfSet(p.Install(cmd.oForce, os.Stdout))
}

func build(cmd *command, p *naos.Project) {
	// build project
	exitIfSet(p.Build(cmd.oClean, cmd.oReconfigure, cmd.oAppOnly, os.Stdout))
}

func detect() {
	// detect devices
	list, err := serial.ListPorts()
	exitIfSet(err)

	// prepare table
	tbl := newTable("PATH")

	// add rows
	for _, d := range list {
		tbl.add(d)
	}

	// show table
	tbl.show(-1)
}

func flash(cmd *command, p *naos.Project) {
	// flash project
	exitIfSet(p.Flash(cmd.aDevice, cmd.oBaudRate, cmd.oErase, cmd.oAppOnly, cmd.oAlt, os.Stdout))
}

func attach(cmd *command, p *naos.Project) {
	// attach to device
	exitIfSet(p.Attach(cmd.aDevice, cmd.oNoReset, os.Stdout, os.Stdin))
}

func run(cmd *command, p *naos.Project) {
	// build project
	exitIfSet(p.Build(cmd.oClean, cmd.oReconfigure, cmd.oAppOnly, os.Stdout))

	// flash project
	exitIfSet(p.Flash(cmd.aDevice, cmd.oBaudRate, cmd.oErase, cmd.oAppOnly, cmd.oAlt, os.Stdout))

	// attach to device
	exitIfSet(p.Attach(cmd.aDevice, false, os.Stdout, os.Stdin))
}

func exec(cmd *command, p *naos.Project) {
	// exec command
	exitIfSet(p.Exec(cmd.aCommand, cmd.aArgs, os.Stdout, os.Stdin))
}

func configFlash(cmd *command, p *naos.Project) {
	// read or write configuration
	if cmd.oRead {
		exitIfSet(p.ReadConfig(cmd.aFile, cmd.aDevice, cmd.oBaudRate, os.Stdout))
	} else {
		exitIfSet(p.Config(cmd.aFile, cmd.aDevice, cmd.oBaudRate, os.Stdout))
	}
}

func format(_ *command, p *naos.Project) {
	// format project
	exitIfSet(p.Format(os.Stdout))
}

func bundle(cmd *command, p *naos.Project) {
	// bundle project
	exitIfSet(p.Bundle(cmd.aFile, cmd.oAddDebug, os.Stdout))
}

func debug(cmd *command, p *naos.Project) {
	// ensure absolute path to ELF file
	if cmd.oELF != "" {
		abs, err := filepath.Abs(cmd.oELF)
		exitIfSet(err)
		cmd.oELF = abs
	}

	// parse or load coredump
	var result []byte
	var err error
	if cmd.aFile != "" {
		// read coredump file
		data, err := os.ReadFile(cmd.aFile)
		exitIfSet(err)

		// parse coredump
		result, err = p.ParseCoredump(cmd.oELF, data)
		exitIfSet(err)
	} else {
		// load and parse coredump from device
		result, err = p.LoadCoredump(cmd.oELF, cmd.aDevice, cmd.oBaudRate, os.Stderr)
		exitIfSet(err)
	}

	// write to file or stdout
	_, err = os.Stdout.Write(result)
	exitIfSet(err)
}

func sdks(cmd *command) {
	// remove the specified version
	if cmd.oRemove != "" {
		_, err := sdk.Remove(cmd.oRemove, os.Stdout)
		exitIfSet(err)
		return
	}

	// list SDKs
	sdks, err := sdk.List()
	exitIfSet(err)

	// prepare table
	tbl := newTable("NAME", "VERSION", "PATH")

	// add rows
	for _, sdk := range sdks {
		tbl.add(sdk.Name, sdk.Version, sdk.Path)
	}

	// show table
	tbl.show(0)
}
