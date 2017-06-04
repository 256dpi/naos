package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"

	"code.cloudfoundry.org/bytefmt"
	"github.com/shiftr-io/nadm"
	"gopkg.in/cheggaaa/pb.v1"
)

func main() {
	cmd := parseCommand()

	if cmd.cCreate {
		create(cmd)
	} else if cmd.cList {
		list(cmd, getInventory(cmd))
	} else if cmd.cCollect {
		collect(cmd, getInventory(cmd))
	} else if cmd.cMonitor {
		monitor(cmd, getInventory(cmd))
	} else if cmd.cUpdate {
		update(cmd, getInventory(cmd))
	} else if cmd.cSet {
		set(cmd, getInventory(cmd))
	} else if cmd.cGet {
		get(cmd, getInventory(cmd))
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

	tbl.string()
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

func monitor(cmd *command, inv *nadm.Inventory) {
	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	tbl := newTable("DEVICE NAME", "DEVICE TYPE", "FIRMWARE VERSION", "FREE HEAP", "UP TIME", "PARTITION")

	err := inv.Monitor(cmd.aFilter, quit, func(d *nadm.Device) {
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

func update(cmd *command, inv *nadm.Inventory) {
	// TODO: Implement multi device update with filters.

	// TODO: Display as table with just a percentage.

	fmt.Println("Reading image...")

	file, err := filepath.Abs(cmd.aImage)
	exitIfSet(err)

	bytes, err := ioutil.ReadFile(file)
	exitIfSet(err)

	fmt.Println("Begin with update...")
	bar := pb.StartNew(len(bytes))

	device, ok := inv.Devices[cmd.aName]
	if !ok {
		exitWithError(fmt.Sprintf("Device with name '%s' not found!", cmd.aName))
	}

	err = nadm.Update(inv.Broker, device.BaseTopic, bytes, func(sent int) { bar.Set(sent) })
	exitIfSet(err)

	bar.Finish()
	fmt.Println("Update finished!")

	save(cmd, inv)
}

func set(cmd *command, inv *nadm.Inventory) {
	list, err := inv.Set(cmd.aFilter, cmd.aParam, cmd.aValue, cmd.oTimeout)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	tbl.string()

	save(cmd, inv)
}

func get(cmd *command, inv *nadm.Inventory) {
	list, err := inv.Get(cmd.aFilter, cmd.aParam, cmd.oTimeout)
	exitIfSet(err)

	tbl := newTable("DEVICE NAME", "PARAM", "VALUE")

	for _, device := range list {
		tbl.add(device.Name, cmd.aParam, device.Parameters[cmd.aParam])
	}

	tbl.string()

	save(cmd, inv)
}

func save(cmd *command, inv *nadm.Inventory) {
	inv.Save(cmd.oInventory)
}
