package main

import (
	"fmt"
	"os"

	"github.com/256dpi/naos/pkg/naos"
	"github.com/256dpi/naos/pkg/sdk"
	"github.com/256dpi/naos/pkg/serial"
)

func main() {
	// parse command
	cmd := parseCommand()

	// run desired command
	if cmd.cCreate {
		create(cmd)
	} else if cmd.cInstall {
		install(cmd, getProject())
	} else if cmd.cBuild {
		build(cmd, getProject())
	} else if cmd.cDetect {
		detect(cmd, getProject())
	} else if cmd.cFlash {
		flash(cmd, getProject())
	} else if cmd.cAttach {
		attach(cmd, getProject())
	} else if cmd.cRun {
		run(cmd, getProject())
	} else if cmd.cExec {
		exec(cmd, getProject())
	} else if cmd.cConfig {
		config(cmd, getProject())
	} else if cmd.cFormat {
		format(cmd, getProject())
	} else if cmd.cBundle {
		bundle(cmd, getProject())
	} else if cmd.cSDKs {
		sdks()
	} else if cmd.cHelp {
		fmt.Print(usage)
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

func detect(cmd *command, p *naos.Project) {
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
	exitIfSet(p.Attach(cmd.aDevice, os.Stdout, os.Stdin))
}

func run(cmd *command, p *naos.Project) {
	// build project
	exitIfSet(p.Build(cmd.oClean, cmd.oReconfigure, cmd.oAppOnly, os.Stdout))

	// flash project
	exitIfSet(p.Flash(cmd.aDevice, cmd.oBaudRate, cmd.oErase, cmd.oAppOnly, cmd.oAlt, os.Stdout))

	// attach to device
	exitIfSet(p.Attach(cmd.aDevice, os.Stdout, os.Stdin))
}

func exec(cmd *command, p *naos.Project) {
	// exec command
	exitIfSet(p.Exec(cmd.aCommand, os.Stdout, os.Stdin))
}

func config(cmd *command, p *naos.Project) {
	// configure device
	exitIfSet(p.Config(cmd.aFile, cmd.aDevice, cmd.oBaudRate, os.Stdout))
}

func format(_ *command, p *naos.Project) {
	// format project
	exitIfSet(p.Format(os.Stdout))
}

func bundle(cmd *command, p *naos.Project) {
	// bundle project
	exitIfSet(p.Bundle(cmd.aFile, os.Stdout))
}

func sdks() {
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
