package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"code.cloudfoundry.org/bytefmt"
	"github.com/shiftr-io/naos/pkg/fleet"
	"github.com/shiftr-io/naos/pkg/naos"
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
	} else if cmd.cInstall {
		install(cmd, getProject())
	} else if cmd.cBuild {
		build(cmd, getProject())
	} else if cmd.cFlash {
		flash(cmd, getProject())
	} else if cmd.cAttach {
		attach(cmd, getProject())
	} else if cmd.cRun {
		run(cmd, getProject())
	} else if cmd.cFormat {
		format(cmd, getProject())
	} else if cmd.cList {
		list(cmd, getProject())
	} else if cmd.cCollect {
		collect(cmd, getProject())
	} else if cmd.cPing {
		ping(cmd, getProject())
	} else if cmd.cSend {
		send(cmd, getProject())
	} else if cmd.cDiscover {
		discover(cmd, getProject())
	} else if cmd.cGet {
		get(cmd, getProject())
	} else if cmd.cSet {
		set(cmd, getProject())
	} else if cmd.cUnset {
		unset(cmd, getProject())
	} else if cmd.cMonitor {
		monitor(cmd, getProject())
	} else if cmd.cRecord {
		record(cmd, getProject())
	} else if cmd.cDebug {
		debug(cmd, getProject())
	} else if cmd.cUpdate {
		update(cmd, getProject())
	} else if cmd.cHelp {
		fmt.Print(usage)
	}
}

func create(cmd *command) {
	// create project
	p, err := naos.CreateProject(workingDirectory(), cmd.oForce, cmd.oCMake, os.Stdout)
	exitIfSet(err)

	// save inventory
	exitIfSet(p.SaveInventory())
}

func install(cmd *command, p *naos.Project) {
	// install dependencies
	exitIfSet(p.Install(cmd.oForce, os.Stdout))
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

func ping(cmd *command, p *naos.Project) {
	// send message
	exitIfSet(p.Inventory.Ping(cmd.aPattern, cmd.oTimeout))
}

func send(cmd *command, p *naos.Project) {
	// send message
	exitIfSet(p.Inventory.Send(cmd.aPattern, cmd.aTopic, cmd.aMessage, cmd.oTimeout))
}

func discover(cmd *command, p *naos.Project) {
	// discover parameters
	list, err := p.Inventory.Discover(cmd.aPattern, cmd.oTimeout)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "PARAMETERS")

	// add rows
	for _, device := range list {
		var list []string
		for p := range device.Parameters {
			list = append(list, p)
		}

		tbl.add(device.Name, strings.Join(list, ", "))
	}

	// show table
	tbl.show(0)

	// show info
	fmt.Printf("\nGot parameters from %d devices.\n", len(list))

	// save inventory
	exitIfSet(p.SaveInventory())
}

func get(cmd *command, p *naos.Project) {
	// get parameter
	list, err := p.Inventory.GetParams(cmd.aPattern, cmd.aParam, cmd.oTimeout)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "VALUE")

	// add rows
	for _, device := range list {
		tbl.add(device.Name, device.Parameters[cmd.aParam])
	}

	// show table
	tbl.show(0)

	// show info
	fmt.Printf("\nGot parameter from %d devices.\n", len(list))

	// save inventory
	exitIfSet(p.SaveInventory())
}

func set(cmd *command, p *naos.Project) {
	// set parameter
	list, err := p.Inventory.SetParams(cmd.aPattern, cmd.aParam, cmd.aValue, cmd.oTimeout)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "VALUE")

	// add rows
	for _, device := range list {
		tbl.add(device.Name, device.Parameters[cmd.aParam])
	}

	// show table
	tbl.show(0)

	// show info
	fmt.Printf("\nSet parameter on %d devices.\n", len(list))

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
	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "FREE HEAP", "UP TIME", "PARTITION", "BATTERY", "SIGNAL STRENGTH")

	// prepare list
	list := make(map[*naos.Device]*fleet.Heartbeat)

	// monitor devices
	exitIfSet(p.Inventory.Monitor(cmd.aPattern, quit, cmd.oTimeout, func(d *naos.Device, hb *fleet.Heartbeat) {
		// set latest heartbeat for device
		list[d] = hb

		// clear previously printed table
		tbl.clear()

		// add rows
		for device, heartbeat := range list {
			// prepare free heap size
			freeHeapSize := bytefmt.ByteSize(uint64(heartbeat.FreeHeapSize))

			// prepare battery level
			var batteryLevel string
			if heartbeat.BatteryLevel >= 0 {
				batteryLevel = strconv.FormatInt(int64(heartbeat.BatteryLevel*100), 10) + "%"
			}

			// prepare signal strength
			var signalStrength string
			if heartbeat.SignalStrength < 0 {
				// map signal strength to percentage
				ss := (100 - (heartbeat.SignalStrength * -1)) * 2
				if ss > 100 {
					ss = 100
				} else if ss < 0 {
					ss = 0
				}

				// format strength
				signalStrength = strconv.FormatInt(ss, 10) + "%"
			}

			// add entry
			tbl.add(device.Name, device.Type, device.FirmwareVersion, freeHeapSize, heartbeat.UpTime.String(), heartbeat.StartPartition, batteryLevel, signalStrength)
		}

		// show table
		tbl.show(0)
	}))

	// save inventory
	exitIfSet(p.SaveInventory())
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

func debug(cmd *command, p *naos.Project) {
	// debug devices
	exitIfSet(p.Debug(cmd.aPattern, cmd.oDelete, cmd.oDuration, os.Stdout))
}

func update(cmd *command, p *naos.Project) {
	// prepare table
	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	// prepare list
	list := make(map[*naos.Device]*fleet.UpdateStatus)

	// update devices
	err := p.Update(cmd.aVersion, cmd.aPattern, cmd.oJobs, cmd.oTimeout, func(d *naos.Device, us *fleet.UpdateStatus) {
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
