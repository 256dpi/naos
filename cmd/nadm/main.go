package main

import (
	"fmt"
	"io/ioutil"

	"github.com/shiftr-io/nadm"
	"gopkg.in/cheggaaa/pb.v1"
)

func main() {
	cmd := parseCommand()

	if cmd.cCollect {
		collect(cmd)
	} else if cmd.cUpdate {
		update(cmd)
	}
}

func collect(cmd *command) {
	fmt.Println("Collecting devices...")

	collector := nadm.NewCollector(cmd.oBrokerURL)

	collector.Start()

	go func() {
		for device := range collector.Devices {
			fmt.Printf("- %s (%s) at %s\n", device.Name, device.Type, device.BaseTopic)
		}
	}()

	waitForInterrupt()
}

func update(cmd *command) {
	fmt.Println("Reading image...")

	bytes, err := ioutil.ReadFile(cmd.aImage)
	exitIfSet(err)

	fmt.Println("Begin with update...")

	updater := nadm.NewUpdater(cmd.oBrokerURL, cmd.aBaseTopic, bytes)

	bar := pb.StartNew(len(bytes))

	updater.OnProgress = func(sent int) {
		bar.Set(sent)
	}

	err = updater.Run()
	exitIfSet(err)

	fmt.Println("Update finished!")
}
