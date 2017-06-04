package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/shiftr-io/nadm"
	"gopkg.in/cheggaaa/pb.v1"
)

func main() {
	cmd := parseCommand()

	if cmd.cCreate {
		create(cmd)
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

	finish(cmd, inv)
}

func collect(cmd *command, inv *nadm.Inventory) {
	fmt.Println("Collecting devices...")

	if cmd.oClear {
		inv.Devices = make(map[string]*nadm.Device)
	}

	list, err := inv.Collect(cmd.oDuration)
	exitIfSet(err)

	fmt.Printf("Found %d new device(s)\n", len(list))

	for _, d := range list {
		fmt.Printf("%s (%s/%s) at %s\n", d.Name, d.Type, d.FirmwareVersion, d.BaseTopic)
	}

	finish(cmd, inv)
}

func monitor(cmd *command, inv *nadm.Inventory) {
	fmt.Printf("Monitoring devices matching '%s'...\n", cmd.aFilter)

	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	err := inv.Monitor(cmd.aFilter, quit, func(d *nadm.Device, hb *nadm.Heartbeat) {
		fmt.Printf("%s (%s/%s), Free Heap Size: %s, Up Time: %s, Start Partition: %s\n", hb.DeviceName, hb.DeviceType, hb.FirmwareVersion, bytefmt.ByteSize(uint64(hb.FreeHeapSize)), hb.UpTime.String(), hb.StartPartition)
	})
	exitIfSet(err)
}

func update(cmd *command, inv *nadm.Inventory) {
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

	finish(cmd, inv)
}

func set(cmd *command, inv *nadm.Inventory) {
	fmt.Printf("Setting '%s' to '%s' on devices matching '%s'\n", cmd.aParam, cmd.aValue, cmd.aFilter)

	var baseTopics []string
	for _, d := range inv.Filter(cmd.aFilter) {
		baseTopics = append(baseTopics, d.BaseTopic)
	}

	err := nadm.Set(inv.Broker, cmd.aParam, cmd.aValue, baseTopics)
	exitIfSet(err)

	fmt.Println("Done!")

	finish(cmd, inv)
}

func get(cmd *command, inv *nadm.Inventory) {
	fmt.Printf("Getting '%s' from devices matching '%s'\n", cmd.aParam, cmd.aFilter)

	var baseTopics []string
	for _, d := range inv.Filter(cmd.aFilter) {
		baseTopics = append(baseTopics, d.BaseTopic)
	}

	table, err := nadm.Get(inv.Broker, cmd.aParam, baseTopics, 1*time.Second)
	exitIfSet(err)

	for baseTopic, value := range table {
		fmt.Printf("%s: %s\n", baseTopic, value)
	}

	fmt.Println("Done!")

	finish(cmd, inv)
}

func finish(cmd *command, inv *nadm.Inventory) {
	inv.Save(cmd.oInventory)
}
