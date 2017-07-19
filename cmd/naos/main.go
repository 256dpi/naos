package main

import (
	"fmt"
	"os"
	"os/signal"

	"code.cloudfoundry.org/bytefmt"
	"github.com/shiftr-io/naos"
	"github.com/shiftr-io/naos/mqtt"
)

func main() {
	// parse command
	cmd := parseCommand()

	// set default pattern
	if cmd.aPattern == "" {
		cmd.aPattern = "*"
	}

	// set default device
	if cmd.aDevice == "" {
		cmd.aDevice = "/dev/cu.SLAB_USBtoUART"
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
	} else if cmd.cRun {
		run(cmd, getProject(cmd))
	} else if cmd.cFormat {
		format(cmd, getProject(cmd))
	} else if cmd.cScan {
		scan(cmd)
	} else if cmd.cRename {
		rename(cmd)
	} else if cmd.cList {
		list(cmd, getProject(cmd))
	} else if cmd.cCollect {
		collect(cmd, getProject(cmd))
	} else if cmd.cGet {
		get(cmd, getProject(cmd))
	} else if cmd.cSet {
		set(cmd, getProject(cmd))
	} else if cmd.cUnset {
		unset(cmd, getProject(cmd))
	} else if cmd.cMonitor {
		monitor(cmd, getProject(cmd))
	} else if cmd.cRecord {
		record(cmd, getProject(cmd))
	} else if cmd.cUpdate {
		update(cmd, getProject(cmd))
	} else if cmd.cHelp {
		fmt.Println(usage)
	}
}

func create(cmd *command) {
	// create project
	p, err := naos.CreateProject(home(), os.Stdout)
	exitIfSet(err)

	// save inventory
	exitIfSet(p.SaveInventory())
}

func setup(cmd *command, p *naos.Project) {
	// setup build tree
	exitIfSet(p.Setup(cmd.oForce, cmd.oCMake, os.Stdout))
}

func build(cmd *command, p *naos.Project) {
	// build project
	exitIfSet(p.Build(cmd.oClean, cmd.oAppOnly, os.Stdout))
}

func flash(cmd *command, p *naos.Project) {
	// flash project
	exitIfSet(p.Flash(cmd.aDevice, cmd.oErase, cmd.oAppOnly, os.Stdout))
}

func attach(cmd *command, p *naos.Project) {
	// attach to device
	exitIfSet(p.Attach(cmd.aDevice, cmd.oSimple, os.Stdout, os.Stdin))
}

func run(cmd *command, p *naos.Project) {
	// build project
	exitIfSet(p.Build(cmd.oClean, cmd.oAppOnly, os.Stdout))

	// flash project
	exitIfSet(p.Flash(cmd.aDevice, cmd.oErase, cmd.oAppOnly, os.Stdout))

	// attach to device
	exitIfSet(p.Attach(cmd.aDevice, cmd.oSimple, os.Stdout, os.Stdin))
}

func format(_ *command, p *naos.Project) {
	// format project
	exitIfSet(p.Format(os.Stdout))
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

func scan(cmd *command) {
	// prepare channel
	quit := make(chan struct{})

	// close channel on interrupt
	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	// scan for configurations
	configurations, err := naos.Scan(cmd.oDuration)
	exitIfSet(err)

	// prepare table
	tbl := newTable("ADDRESS", "DEVICE NAME", "DEVICE TYPE", "BASE TOPIC", "WIFI SSID", "WIFI PASSWORD", "MQTT HOST", "MQTT PORT", "MQTT CLIENT ID", "MQTT USERNAME", "MQTT PASSWORD", "CONNECTION STATUS")

	// add rows
	for addr, d := range configurations {
		tbl.add(addr, d.DeviceName, d.DeviceType, d.BaseTopic, d.WiFiSSID, d.WiFiPassword, d.MQTTHost, d.MQTTPort, d.MQTTClientID, d.MQTTUsername, d.MQTTPassword, d.ConnectionStatus)
	}

	// show table
	tbl.show(0)
}

func rename(cmd *command) {
	// rename device
	err := naos.Rename(cmd.aAddress, cmd.aName)
	exitIfSet(err)
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
	exitIfSet(p.SaveInventory())
}

func get(cmd *command, p *naos.Project) {
	// get parameter
	list, err := p.Inventory.GetParams(cmd.aPattern, cmd.aParam, cmd.oTimeout)
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
	exitIfSet(p.SaveInventory())
}

func set(cmd *command, p *naos.Project) {
	// set parameter
	list, err := p.Inventory.SetParams(cmd.aPattern, cmd.aParam, cmd.aValue, cmd.oTimeout)
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
	exitIfSet(p.SaveInventory())
}

func unset(cmd *command, p *naos.Project) {
	// unset parameter
	_, err := p.Inventory.UnsetParams(cmd.aPattern, cmd.aParam, cmd.oTimeout)
	exitIfSet(err)

	// save inventory
	exitIfSet(p.SaveInventory())
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

	// prepare list
	list := make(map[*naos.Device]*mqtt.Heartbeat)

	// monitor devices
	exitIfSet(p.Inventory.Monitor(cmd.aPattern, quit, cmd.oTimeout, func(d *naos.Device, hb *mqtt.Heartbeat) {
		// set latest heartbeat for device
		list[d] = hb

		// clear previously printed table
		tbl.clear()

		// add rows
		for device, heartbeat := range list {
			tbl.add(device.Name, device.Type, device.FirmwareVersion, bytefmt.ByteSize(uint64(heartbeat.FreeHeapSize)), heartbeat.UpTime.String(), heartbeat.StartPartition)
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
	exitIfSet(p.Inventory.Record(cmd.aPattern, quit, cmd.oTimeout, func(d *naos.Device, msg string) {
		// show log message
		fmt.Printf("[%s] %s\n", d.Name, msg)
	}))
}

func update(cmd *command, p *naos.Project) {
	// prepare table
	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	// prepare list
	list := make(map[*naos.Device]*mqtt.UpdateStatus)

	// update devices
	err := p.Update(cmd.aPattern, cmd.oTimeout, func(d *naos.Device, us *mqtt.UpdateStatus) {
		// save status
		list[d] = us

		// clear previously printed table
		tbl.clear()

		// add rows
		for device, status := range list {
			// get error string if set
			errStr := ""
			if status.Error != nil {
				errStr = status.Error.Error()
			}

			// add row
			tbl.add(device.Name, fmt.Sprintf("%.2f%%", status.Progress*100), errStr)
		}

		// show table
		tbl.show(0)
	})

	// save inventory
	exitIfSet(p.SaveInventory())

	// check error
	exitIfSet(err)
}
