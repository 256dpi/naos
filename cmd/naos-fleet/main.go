package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"

	"github.com/256dpi/naos/pkg/fleet"
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
		create()
	} else if cmd.cList {
		list(cmd, getFleet())
	} else if cmd.cCollect {
		collect(cmd, getFleet())
	} else if cmd.cDiscover {
		discover(cmd, getFleet())
	} else if cmd.cPing {
		ping(cmd, getFleet())
	} else if cmd.cGet {
		get(cmd, getFleet())
	} else if cmd.cSet {
		set(cmd, getFleet())
	} else if cmd.cUnset {
		unset(cmd, getFleet())
	} else if cmd.cMonitor {
		monitor(cmd, getFleet())
	} else if cmd.cRecord {
		record(cmd, getFleet())
	} else if cmd.cDebug {
		debug(cmd, getFleet())
	} else if cmd.cUpdate {
		update(cmd, getFleet())
	} else if cmd.cHelp {
		fmt.Print(usage)
	}
}

func create() {
	// create new fleet
	saveFleet(fleet.NewFleet())

	// log info
	fmt.Println("Created new fleet in 'fleet.json'.")
}

func list(_ *command, f *fleet.Fleet) {
	// prepare table
	tbl := newTable("DEVICE NAME", "APP NAME", "APP VERSION", "BASE TOPIC")

	// add rows
	for _, d := range f.Devices {
		tbl.add(d.DeviceName, d.AppName, d.AppVersion, d.BaseTopic)
	}

	// show table
	tbl.show(0)
}

func collect(cmd *command, f *fleet.Fleet) {
	// clear all previously collected devices
	if cmd.oClear {
		f.Devices = make(map[string]*fleet.Device)
	}

	// collect devices
	list, err := f.Collect(cmd.oDuration)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "APP NAME", "APP VERSION", "BASE TOPIC")

	// add rows
	for _, d := range list {
		tbl.add(d.DeviceName, d.AppName, d.AppVersion, d.BaseTopic)
	}

	// show table
	tbl.show(0)

	// save fleet
	saveFleet(f)
}

func discover(cmd *command, f *fleet.Fleet) {
	// discover parameters
	list, err := f.Discover(cmd.aPattern, cmd.oJobs)
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

	// save fleet
	saveFleet(f)
}

func ping(cmd *command, f *fleet.Fleet) {
	// ping devices
	exitIfSet(f.Ping(cmd.aPattern, cmd.oJobs))
}

func get(cmd *command, f *fleet.Fleet) {
	// get parameter
	list, err := f.GetParams(cmd.aPattern, cmd.aParam, cmd.oJobs)
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

	// save fleet
	saveFleet(f)
}

func set(cmd *command, f *fleet.Fleet) {
	// set parameter
	list, err := f.SetParams(cmd.aPattern, cmd.aParam, cmd.aValue, cmd.oJobs)
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

	// save fleet
	saveFleet(f)
}

func unset(cmd *command, f *fleet.Fleet) {
	// unset parameter
	_, err := f.UnsetParams(cmd.aPattern, cmd.aParam, cmd.oJobs)
	exitIfSet(err)

	// save fleet
	saveFleet(f)
}

func monitor(cmd *command, f *fleet.Fleet) {
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
	exitIfSet(f.Monitor(cmd.aPattern, quit, func(d *fleet.Device, hb *fleet.Heartbeat) {
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

	// save fleet
	saveFleet(f)
}

func record(cmd *command, f *fleet.Fleet) {
	// prepare channel
	quit := make(chan struct{})

	// close channel on interrupt
	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	// record devices
	start := time.Now()
	err := f.Record(cmd.aPattern, quit, func(t time.Time, d *fleet.Device, msg string) {
		fmt.Printf("%s [%s] %s\n", time.Since(start).Round(time.Millisecond).String(), d.DeviceName, msg)
	})
	exitIfSet(err)
}

func debug(cmd *command, f *fleet.Fleet) {
	// debug devices
	coredumps, err := f.Debug(cmd.aPattern, cmd.oDelete, cmd.oJobs)
	exitIfSet(err)

	// ensure directory
	err = os.MkdirAll(filepath.Join(workingDirectory(), "debug"), 0755)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "FILE")

	// add rows
	for device, coredump := range coredumps {
		// TODO: Support coredump parsing again.

		// parse coredump
		// coredump, err := tree.ParseCoredump(tree, name, coredump)
		// exitIfSet(err)

		// prepare path
		path := filepath.Join(workingDirectory(), "debug", device.DeviceName+".bin")

		// write parsed data
		err = os.WriteFile(path, coredump, 0644)
		exitIfSet(err)

		// add row
		tbl.add(device.DeviceName, path)
	}

	// show table
	tbl.show(0)

	// log info
	fmt.Printf("\nGot %d coredump(s)\n", len(coredumps))
}

func update(cmd *command, f *fleet.Fleet) {
	// read binary
	binary, err := os.ReadFile(cmd.aFile)
	exitIfSet(err)

	// prepare table
	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	// prepare list
	list := make(map[*fleet.Device]fleet.UpdateStatus)

	// update devices
	err = f.Update(cmd.aVersion, cmd.aPattern, binary, cmd.oJobs, func(d *fleet.Device, us fleet.UpdateStatus) {
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
	exitIfSet(err)
}
