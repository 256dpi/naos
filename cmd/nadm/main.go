package main

import (
	"fmt"
	"io/ioutil"

	"github.com/shiftr-io/nadm"
	"gopkg.in/cheggaaa/pb.v1"
)

func main() {
	cmd := parseCommand()

	m := nadm.NewManager(cmd.oBrokerURL)

	if cmd.cCollect {
		collect(m, cmd)
	} else if cmd.cUpdate {
		update(m, cmd)
	}
}

func collect(m *nadm.Manager, cmd *command) {
	fmt.Println("Collecting device announcements...")

	list, err := m.CollectAnnouncements(cmd.oDuration)
	exitIfSet(err)

	for _, a := range list {
		fmt.Printf("- %s (%s/%s) at %s\n", a.Name, a.Type, a.Version, a.BaseTopic)
	}
}

func update(m *nadm.Manager, cmd *command) {
	fmt.Println("Reading image...")

	bytes, err := ioutil.ReadFile(cmd.aImage)
	exitIfSet(err)

	fmt.Println("Begin with update...")
	bar := pb.StartNew(len(bytes))

	err = m.UpdateFirmware(cmd.aBaseTopic, bytes, func(sent int) { bar.Set(sent) })
	exitIfSet(err)

	bar.Finish()
	fmt.Println("Update finished!")
}
