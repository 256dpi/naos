package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"

	"github.com/256dpi/naos/pkg/fleet"
	"github.com/256dpi/naos/pkg/naos"
	"github.com/256dpi/naos/pkg/sdk"
	"github.com/256dpi/naos/pkg/serial"
)

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
	} else if cmd.cList {
		list(cmd, getProject())
	} else if cmd.cCollect {
		collect(cmd, getProject())
	} else if cmd.cDiscover {
		discover(cmd, getProject())
	} else if cmd.cPing {
		ping(cmd, getProject())
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

func list(_ *command, p *naos.Project) {
	// prepare table
	tbl := newTable("DEVICE NAME", "APP NAME", "APP VERSION", "BASE TOPIC")

	// add rows
	for _, d := range p.Fleet.Devices {
		tbl.add(d.DeviceName, d.AppName, d.AppVersion, d.BaseTopic)
	}

	// show table
	tbl.show(0)
}

func collect(cmd *command, p *naos.Project) {
	// clear all previously collected devices
	if cmd.oClear {
		p.Fleet.Devices = make(map[string]*fleet.Device)
	}

	// collect devices
	list, err := p.Fleet.Collect(cmd.oDuration)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "APP NAME", "APP VERSION", "BASE TOPIC")

	// add rows
	for _, d := range list {
		tbl.add(d.DeviceName, d.AppName, d.AppVersion, d.BaseTopic)
	}

	// show table
	tbl.show(0)

	// save inventory
	exitIfSet(p.SaveFleet())
}

func discover(cmd *command, p *naos.Project) {
	// discover parameters
	list, err := p.Fleet.Discover(cmd.aPattern, cmd.oJobs)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "PARAMETERS")

	// add rows
	for _, device := range list {
		var list []string
		for p := range device.Parameters {
			list = append(list, p)
		}

		tbl.add(device.DeviceName, strings.Join(list, ", "))
	}

	// show table
	tbl.show(0)

	// show info
	fmt.Printf("\nGot parameters from %d devices.\n", len(list))

	// save inventory
	exitIfSet(p.SaveFleet())
}

func ping(cmd *command, p *naos.Project) {
	// ping devices
	exitIfSet(p.Fleet.Ping(cmd.aPattern, cmd.oJobs))
}

func get(cmd *command, p *naos.Project) {
	// get parameter
	list, err := p.Fleet.GetParams(cmd.aPattern, cmd.aParam, cmd.oJobs)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "VALUE")

	// add rows
	for _, device := range list {
		tbl.add(device.DeviceName, device.Parameters[cmd.aParam])
	}

	// show table
	tbl.show(0)

	// show info
	fmt.Printf("\nGot parameter from %d devices.\n", len(list))

	// save inventory
	exitIfSet(p.SaveFleet())
}

func set(cmd *command, p *naos.Project) {
	// set parameter
	list, err := p.Fleet.SetParams(cmd.aPattern, cmd.aParam, cmd.aValue, cmd.oJobs)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "VALUE")

	// add rows
	for _, device := range list {
		tbl.add(device.DeviceName, device.Parameters[cmd.aParam])
	}

	// show table
	tbl.show(0)

	// show info
	fmt.Printf("\nSet parameter on %d devices.\n", len(list))

	// save inventory
	exitIfSet(p.SaveFleet())
}

func unset(cmd *command, p *naos.Project) {
	// unset parameter
	_, err := p.Fleet.UnsetParams(cmd.aPattern, cmd.aParam, cmd.oJobs)
	exitIfSet(err)

	// save inventory
	exitIfSet(p.SaveFleet())
}

func monitor(cmd *command, p *naos.Project) {
	// prepare channel
	quit := make(chan struct{})

	// close channel on interrupt
	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	// prepare table
	tbl := newTable(
		"DEVICE NAME",
		"APP NAME",
		"APP VERSION",
		"APP PARTITION",
		"FREE ALL",
		"FREE INT",
		"FREE EXT",
		"UP TIME",
		"BATTERY",
		"WIFI",
		"CPU0",
		"CPU1",
	)

	// prepare list
	list := make(map[*fleet.Device]*fleet.Heartbeat)

	// monitor devices
	exitIfSet(p.Fleet.Monitor(cmd.aPattern, quit, func(d *fleet.Device, hb *fleet.Heartbeat) {
		// set the latest heartbeat for device
		list[d] = hb

		// clear previously printed table
		tbl.clear()

		// add rows
		for device, heartbeat := range list {
			// prepare free memory
			freeMemoryAll := bytefmt.ByteSize(uint64(heartbeat.FreeMemory[0]))
			freeMemoryInternal := bytefmt.ByteSize(uint64(heartbeat.FreeMemory[1]))
			freeMemoryExternal := bytefmt.ByteSize(uint64(heartbeat.FreeMemory[2]))

			// prepare uptime
			uptime := heartbeat.UpTime.Truncate(time.Second).String()

			// prepare battery level
			batteryLevel := "n/a"
			if heartbeat.BatteryLevel >= 0 {
				batteryLevel = strconv.FormatInt(int64(heartbeat.BatteryLevel*100), 10) + "%"
			}

			// prepare signal strength
			signalStrength := "n/a"
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

			// prepare CPU usages
			cpu0Usage := strconv.FormatFloat(heartbeat.CPUUsage[0]*100, 'f', 0, 64) + "%"
			cpu1Usage := strconv.FormatFloat(heartbeat.CPUUsage[1]*100, 'f', 0, 64) + "%"

			// add entry
			tbl.add(
				device.DeviceName,
				device.AppName,
				device.AppVersion,
				heartbeat.AppPartition,
				freeMemoryAll, freeMemoryInternal,
				freeMemoryExternal,
				uptime,
				batteryLevel,
				signalStrength,
				cpu0Usage,
				cpu1Usage,
			)
		}

		// show table
		tbl.show(0)
	}))

	// save inventory
	exitIfSet(p.SaveFleet())
}

func record(cmd *command, p *naos.Project) {
	// prepare channel
	quit := make(chan struct{})

	// close channel on interrupt
	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	// get start
	start := time.Now()

	// record devices
	exitIfSet(p.Fleet.Record(cmd.aPattern, quit, func(t time.Time, d *fleet.Device, msg string) {
		// show log message
		fmt.Printf("%s [%s] %s\n", time.Since(start).Round(time.Millisecond).String(), d.DeviceName, msg)
	}))
}

func debug(cmd *command, p *naos.Project) {
	// debug devices
	exitIfSet(p.Debug(cmd.aPattern, cmd.oDelete, cmd.oJobs, os.Stdout))
}

func update(cmd *command, p *naos.Project) {
	// prepare table
	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	// prepare list
	list := make(map[*fleet.Device]fleet.UpdateStatus)

	// update devices
	err := p.Update(cmd.aVersion, cmd.aPattern, cmd.oJobs, func(d *fleet.Device, us fleet.UpdateStatus) {
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
			tbl.add(device.DeviceName, fmt.Sprintf("%.2f%%", status.Progress*100), errStr)
		}

		// show table
		tbl.show(0)
	})

	// save inventory
	exitIfSet(p.SaveFleet())

	// check error
	exitIfSet(err)
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
