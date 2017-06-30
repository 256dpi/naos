package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/bytefmt"
	"github.com/shiftr-io/naos"
)

// TODO: Buffer output if not verbose and print on error.

func main() {
	// parse command
	cmd := parseCommand()

	// set default pattern
	if cmd.aPattern == "" {
		cmd.aPattern = "*"
	}

	// run desired command
	if cmd.cCreate {
		create(cmd)
	} else if cmd.cSetup {
		setup(cmd, getProject(cmd))
	} else if cmd.cBuild {
		build(cmd, getProject(cmd))
	} else if cmd.cFlash {
		flash(cmd, getProject(cmd))
	} else if cmd.cAttach {
		attach(cmd, getProject(cmd))
	} else if cmd.cFormat {
		format(cmd, getProject(cmd))
	} else if cmd.cList {
		list(cmd, getProject(cmd))
	} else if cmd.cCollect {
		collect(cmd, getProject(cmd))
	} else if cmd.cSet {
		set(cmd, getProject(cmd))
	} else if cmd.cGet {
		get(cmd, getProject(cmd))
	} else if cmd.cMonitor {
		monitor(cmd, getProject(cmd))
	} else if cmd.cRecord {
		record(cmd, getProject(cmd))
	} else if cmd.cUpdate {
		update(cmd, getProject(cmd))
	}
}

func create(cmd *command) {
	// create project
	p, err := naos.CreateProject(home(), getOutput(cmd))
	exitIfSet(err)

	// save inventory
	save(cmd, p)
}

func setup(cmd *command, p *naos.Project) {
	// setup toolchain
	exitIfSet(p.SetupToolchain(cmd.oForce, getOutput(cmd)))

	// setup development framework
	exitIfSet(p.SetupDevelopmentFramework(cmd.oForce, getOutput(cmd)))

	// setup build tree
	exitIfSet(p.SetupBuildTree(cmd.oForce, getOutput(cmd)))

	// setup cmake if required
	if cmd.oCMake {
		exitIfSet(p.SetupCMake(cmd.oForce, getOutput(cmd)))
	}
}

func build(cmd *command, p *naos.Project) {
	// build project
	exitIfSet(p.Build(cmd.oAppOnly, getOutput(cmd)))
}

func flash(cmd *command, p *naos.Project) {
	// flash project
	exitIfSet(p.Flash(cmd.oErase, cmd.oAppOnly, getOutput(cmd)))
}

func attach(_ *command, p *naos.Project) {
	// attach to device
	exitIfSet(p.Attach(os.Stdout, os.Stdin))
}

func format(cmd *command, p *naos.Project) {
	// format project
	exitIfSet(p.Format(getOutput(cmd)))
}

func list(_ *command, p *naos.Project) {
	// prepare table
	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "BASE TOPIC")

	// add rows
	for _, d := range p.Inventory.Devices {
		tbl.add(d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	// show table
	tbl.show(0)
}

func collect(cmd *command, p *naos.Project) {
	// clear all previously collected devices
	if cmd.oClear {
		p.Inventory.Devices = make(map[string]*naos.Device)
	}

	// collect devices
	list, err := p.Inventory.Collect(cmd.oDuration)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "BASE TOPIC")

	// add rows
	for _, d := range list {
		tbl.add(d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	// show table
	tbl.show(0)

	// save inventory
	save(cmd, p)
}

func set(cmd *command, p *naos.Project) {
	// set parameter
	list, err := p.Inventory.Set(cmd.aPattern, cmd.aParam, cmd.aValue, cmd.oTimeout)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	// add rows
	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	// show table
	tbl.show(0)

	// save inventory
	save(cmd, p)
}

func get(cmd *command, p *naos.Project) {
	// get parameter
	list, err := p.Inventory.Get(cmd.aPattern, cmd.aParam, cmd.oTimeout)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	// add rows
	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	// show table
	tbl.show(0)

	// save inventory
	save(cmd, p)
}

func monitor(cmd *command, p *naos.Project) {
	// prepare channel
	quit := make(chan struct{})

	// close channel on interrupt
	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	// prepare table
	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "FREE HEAP", "UP TIME", "PARTITION")

	// monitor devices
	exitIfSet(p.Inventory.Monitor(cmd.aPattern, quit, func(d *naos.Device) {
		// clear previously printed table
		tbl.clear()

		// add rows
		for _, device := range p.Inventory.Devices {
			if device.LastHeartbeat != nil {
				tbl.add(device.Name, device.Type, device.FirmwareVersion, bytefmt.ByteSize(uint64(device.LastHeartbeat.FreeHeapSize)), device.LastHeartbeat.UpTime.String(), device.LastHeartbeat.StartPartition)
			}
		}

		// show table
		tbl.show(0)
	}))
}

func record(cmd *command, p *naos.Project) {
	// prepare channel
	quit := make(chan struct{})

	// close channel on interrupt
	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	// record devices
	exitIfSet(p.Inventory.Record(cmd.aPattern, quit, func(d *naos.Device, msg string) {
		// show log message
		fmt.Printf("[%s] %s\n", d.Name, msg)
	}))
}

func update(cmd *command, p *naos.Project) {
	// get absolute image path
	file, err := filepath.Abs(cmd.aImage)
	exitIfSet(err)

	// read image
	bytes, err := ioutil.ReadFile(file)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	// update devices
	p.Inventory.Update(cmd.aPattern, bytes, cmd.oTimeout, func(_ *naos.Device) {
		// clear previously printed table
		tbl.clear()

		// add rows
		for _, device := range p.Inventory.Devices {
			if device.UpdateStatus != nil {
				errStr := ""
				if device.UpdateStatus.Error != nil {
					errStr = device.UpdateStatus.Error.Error()
				}

				progress := 100.0 / float64(len(bytes)) * float64(device.UpdateStatus.Progress)

				tbl.add(device.Name, strconv.Itoa(int(progress))+"%", errStr)
			}
		}

		// show table
		tbl.show(0)
	})

	// save inventory
	save(cmd, p)
}

func save(_ *command, p *naos.Project) {
	// save inventory
	exitIfSet(p.SaveInventory())
}
