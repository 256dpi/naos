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

func main() {
	cmd := parseCommand()

	if cmd.aPattern == "" {
		cmd.aPattern = "*"
	}

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
	p, err := naos.CreateProject(home())
	exitIfSet(err)

	// set broker url
	p.Inventory.Broker = "mqtts://key:secret@broker.shiftr.io"

	fmt.Printf("Created a new project at '%s'.\n", p.Location)

	fmt.Println("Please add your MQTT broker credentials to proceed.")

	save(cmd, p)
}

func setup(cmd *command, p *naos.Project) {
	// setup toolchain
	err := p.SetupToolchain(cmd.oForce, getOutput(cmd))
	exitIfSet(err)

	// setup development framework
	err = p.SetupDevelopmentFramework(cmd.oForce, getOutput(cmd))
	exitIfSet(err)

	// setup build tree
	err = p.SetupBuildTree(cmd.oForce, getOutput(cmd))
	exitIfSet(err)
}

func build(cmd *command, p *naos.Project) {
	// build project
	err := p.Build(cmd.oAppOnly, getOutput(cmd))
	exitIfSet(err)
}

func flash(cmd *command, p *naos.Project) {
	// flash project
	err := p.Flash(cmd.oErase, cmd.oAppOnly, getOutput(cmd))
	exitIfSet(err)
}

func attach(cmd *command, p *naos.Project) {
	// TODO: Run serial tool from esp tools.
}

func list(cmd *command, p *naos.Project) {
	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "BASE TOPIC")

	for _, d := range p.Inventory.Devices {
		tbl.add(d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	tbl.print()
}

func collect(cmd *command, p *naos.Project) {
	if cmd.oClear {
		p.Inventory.Devices = make(map[string]*naos.Device)
	}

	list, err := p.Inventory.Collect(cmd.oDuration)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "BASE TOPIC")

	for _, d := range list {
		tbl.add(d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	tbl.print()

	save(cmd, p)
}

func set(cmd *command, p *naos.Project) {
	list, err := p.Inventory.Set(cmd.aPattern, cmd.aParam, cmd.aValue, cmd.oTimeout)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	tbl.print()

	save(cmd, p)
}

func get(cmd *command, p *naos.Project) {
	list, err := p.Inventory.Get(cmd.aPattern, cmd.aParam, cmd.oTimeout)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	tbl.print()

	save(cmd, p)
}

func monitor(cmd *command, p *naos.Project) {
	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "FREE HEAP", "UP TIME", "PARTITION")

	err := p.Inventory.Monitor(cmd.aPattern, quit, func(d *naos.Device) {
		tbl.clear()

		for _, device := range p.Inventory.Devices {
			if device.LastHeartbeat != nil {
				tbl.add(device.Name, device.Type, device.FirmwareVersion, bytefmt.ByteSize(uint64(device.LastHeartbeat.FreeHeapSize)), device.LastHeartbeat.UpTime.String(), device.LastHeartbeat.StartPartition)
			}
		}

		tbl.print()
	})
	exitIfSet(err)
}

func record(cmd *command, p *naos.Project) {
	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	err := p.Inventory.Record(cmd.aPattern, quit, func(d *naos.Device, msg string) {
		fmt.Printf("[%s] %s\n", d.Name, msg)
	})
	exitIfSet(err)
}

func update(cmd *command, p *naos.Project) {
	file, err := filepath.Abs(cmd.aImage)
	exitIfSet(err)

	bytes, err := ioutil.ReadFile(file)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	p.Inventory.Update(cmd.aPattern, bytes, cmd.oTimeout, func(_ *naos.Device) {
		tbl.clear()

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

		tbl.print()
	})

	save(cmd, p)
}

func save(cmd *command, p *naos.Project) {
	err := p.SaveInventory()
	exitIfSet(err)
}
