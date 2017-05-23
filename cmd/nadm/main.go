package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/shiftr-io/nadm"
	"gopkg.in/cheggaaa/pb.v1"
)

func main() {
	cmd := parseCommand()

	if cmd.cCreate {
		create(cmd)
	} else if cmd.cCollect {
		collect(cmd, getInventory(cmd))
	} else if cmd.cUpdate {
		update(cmd, getInventory(cmd))
	}
}

func create(cmd *command) {
	fmt.Printf("Creating a new inventory at '%s'...\n", cmd.oInventory)

	fmt.Println("Please add your MQTT broker credentials.")

	inv := &nadm.Inventory{
		Broker: "mqtts://key:secret@broker.shiftr.io",
	}

	finish(cmd, inv)
}

func collect(cmd *command, inv *nadm.Inventory) {
	fmt.Println("Collecting devices...")

	m := nadm.NewManager(inv.Broker)

	list, err := m.CollectAnnouncements(cmd.oDuration)
	exitIfSet(err)

	if cmd.oClear {
		inv.Devices = make(map[string]*nadm.Device)
	}

	for _, a := range list {
		inv.Devices[a.Name] = &nadm.Device{
			Name:      a.Name,
			Type:      a.Type,
			Version:   a.Version,
			BaseTopic: a.BaseTopic,
		}

		fmt.Printf("Found: %s (%s/%s) at %s\n", a.Name, a.Type, a.Version, a.BaseTopic)
	}

	finish(cmd, inv)
}

func update(cmd *command, inv *nadm.Inventory) {
	fmt.Println("Reading image...")

	m := nadm.NewManager(inv.Broker)

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

	err = m.UpdateFirmware(device.BaseTopic, bytes, func(sent int) { bar.Set(sent) })
	exitIfSet(err)

	bar.Finish()
	fmt.Println("Update finished!")

	finish(cmd, inv)
}

func finish(cmd *command, inv *nadm.Inventory) {
	inv.Save(cmd.oInventory)
}
