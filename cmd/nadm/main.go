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
	} else if cmd.cCollect {
		collect(cmd, getInventory(cmd))
	} else if cmd.cMonitor {
		monitor(cmd, getInventory(cmd))
	} else if cmd.cUpdate {
		update(cmd, getInventory(cmd))
	}
}

func create(cmd *command) {
	fmt.Printf("Creating a new inventory at '%s'...\n", cmd.oInventory)

	fmt.Println("Please add your MQTT broker credentials.")

	inv := nadm.NewInventory("mqtts://key:secret@broker.shiftr.io")

	finish(cmd, inv)
}

func collect(cmd *command, inv *nadm.Inventory) {
	fmt.Println("Collecting devices...")

	list, err := nadm.CollectAnnouncements(inv.Broker, cmd.oDuration)
	exitIfSet(err)

	if cmd.oClear {
		inv.Devices = make(map[string]string)
	}

	for _, a := range list {
		inv.Devices[a.Name] = a.BaseTopic

		fmt.Printf("Found: %s (%s/%s) at %s\n", a.Name, a.Type, a.Version, a.BaseTopic)
	}

	finish(cmd, inv)
}

func monitor(cmd *command, inv *nadm.Inventory) {
	fmt.Println("Monitoring all devices...")

	quit := make(chan struct{})

	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Interrupt)
		<-exit
		close(quit)
	}()

	var baseTopics []string

	for _, baseTopic := range inv.Devices {
		baseTopics = append(baseTopics, baseTopic)
	}

	err := nadm.MonitorDevices(inv.Broker, baseTopics, quit, func(hb *nadm.Heartbeat) {
		fmt.Printf("Device %s (%s/%s), Free Heap Size: %s, Up Time: %s, Start Partition: %s\n", hb.DeviceName, hb.DeviceType, hb.FirmwareVersion, bytefmt.ByteSize(uint64(hb.FreeHeapSize)), hb.UpTime.String(), hb.StartPartition)
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

	baseTopic, ok := inv.Devices[cmd.aName]
	if !ok {
		exitWithError(fmt.Sprintf("Device with name '%s' not found!", cmd.aName))
	}

	err = nadm.UpdateFirmware(inv.Broker, baseTopic, bytes, func(sent int) { bar.Set(sent) })
	exitIfSet(err)

	bar.Finish()
	fmt.Println("Update finished!")

	finish(cmd, inv)
}

func finish(cmd *command, inv *nadm.Inventory) {
	inv.Save(cmd.oInventory)
}
