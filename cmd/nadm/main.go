package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/bytefmt"
	"github.com/shiftr-io/nadm"
)

func main() {
	cmd := parseCommand()

	if cmd.cCreate {
		create(cmd)
	} else if cmd.cList {
		list(cmd, getInventory(cmd))
	} else if cmd.cCollect {
		collect(cmd, getInventory(cmd))
	} else if cmd.cSet {
		set(cmd, getInventory(cmd))
	} else if cmd.cGet {
		get(cmd, getInventory(cmd))
	} else if cmd.cMonitor {
		monitor(cmd, getInventory(cmd))
	} else if cmd.cRecord {
		record(cmd, getInventory(cmd))
	} else if cmd.cUpdate {
		update(cmd, getInventory(cmd))
	}
}

func create(cmd *command) {
	inv := nadm.NewInventory("mqtts://key:secret@broker.shiftr.io")

	fmt.Printf("Created a new inventory at '%s'.\n", cmd.oInventory)

	fmt.Println("Please add your MQTT broker credentials to proceed.")

	save(cmd, inv)
}

func list(cmd *command, inv *nadm.Inventory) {
	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "BASE TOPIC")

	for _, d := range inv.Devices {
		tbl.add(d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	tbl.print()
}

func collect(cmd *command, inv *nadm.Inventory) {
	if cmd.oClear {
		inv.Devices = make(map[string]*nadm.Device)
	}

	list, err := inv.Collect(cmd.oDuration)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "BASE TOPIC")

	for _, d := range list {
		tbl.add(d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	tbl.print()

	save(cmd, inv)
}

func set(cmd *command, inv *nadm.Inventory) {
	list, err := inv.Set(cmd.oFilter, cmd.aParam, cmd.aValue, cmd.oTimeout)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	tbl.print()

	save(cmd, inv)
}

func get(cmd *command, inv *nadm.Inventory) {
	list, err := inv.Get(cmd.oFilter, cmd.aParam, cmd.oTimeout)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	tbl.print()

	save(cmd, inv)
}

func monitor(cmd *command, inv *nadm.Inventory) {
	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "FREE HEAP", "UP TIME", "PARTITION")

	err := inv.Monitor(cmd.oFilter, quit, func(d *nadm.Device) {
		tbl.clear()

		for _, device := range inv.Devices {
			if device.LastHeartbeat != nil {
				tbl.add(device.Name, device.Type, device.FirmwareVersion, bytefmt.ByteSize(uint64(device.LastHeartbeat.FreeHeapSize)), device.LastHeartbeat.UpTime.String(), device.LastHeartbeat.StartPartition)
			}
		}

		tbl.print()
	})
	exitIfSet(err)
}

func record(cmd *command, inv *nadm.Inventory) {
	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	err := inv.Record(cmd.oFilter, quit, func(d *nadm.Device, msg string) {
		fmt.Printf("[%s] %s\n", d.Name, msg)
	})
	exitIfSet(err)
}

func update(cmd *command, inv *nadm.Inventory) {
	file, err := filepath.Abs(cmd.aImage)
	exitIfSet(err)

	bytes, err := ioutil.ReadFile(file)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PROGRESS", "ERROR")

	inv.Update(cmd.oFilter, bytes, cmd.oTimeout, func(_ *nadm.Device){
		tbl.clear()

		for _, device := range inv.Devices {
			if device.UpdateStatus != nil {
				errStr := ""
				if device.UpdateStatus.Error != nil {
					errStr = device.UpdateStatus.Error.Error()
				}

				progress := 100.0 / float64(len(bytes)) * float64(device.UpdateStatus.Progress)

				tbl.add(device.Name, strconv.Itoa(int(progress)) + "%", errStr)
			}
		}

		tbl.print()
	})

	save(cmd, inv)
}

func save(cmd *command, inv *nadm.Inventory) {
	inv.Save(cmd.oInventory)
}
