package fleet

import (
	"strconv"
	"strings"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// A Heartbeat is a single heartbeat emitted by Monitor.
type Heartbeat struct {
	DeviceName      string
	DeviceType      string
	FirmwareVersion string
	FreeHeapSize    int
	UpTime          time.Duration
	StartPartition  string
}

// Monitor will listen to the passed base topics for heartbeats and call the
// supplied callback until the specified quit channel is closed.
//
// Note: Not correctly formatted heartbeats are ignored.
func Monitor(url string, baseTopics []string, quit chan struct{}, cb func(*Heartbeat)) error {
	// prepare channels
	errs := make(chan error)

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) {
		// send errors
		if err != nil {
			errs <- err
			return
		}

		// get data from payload
		data := strings.Split(string(msg.Payload), ",")

		// check length
		if len(data) < 6 {
			return
		}

		// convert integers
		freeHeapSize, _ := strconv.Atoi(data[3])
		upTime, _ := strconv.Atoi(data[4])

		// call callback
		cb(&Heartbeat{
			DeviceType:      data[0],
			FirmwareVersion: data[1],
			DeviceName:      data[2],
			FreeHeapSize:    freeHeapSize,
			UpTime:          time.Duration(upTime) * time.Millisecond,
			StartPartition:  data[5],
		})
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return err
	}

	// wait for ack
	err = cf.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	// make sure client gets closed
	defer cl.Close()

	// prepare subscriptions
	var subs []packet.Subscription

	// add subscriptions
	for _, baseTopic := range baseTopics {
		subs = append(subs, packet.Subscription{
			Topic: baseTopic + "/nadk/heartbeat",
			QOS:   0,
		})
	}

	// subscribe to next chunk topic
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	// wait for error or quit
	select {
	case err = <-errs:
		return err
	case <-quit:
		// move on
	}

	// disconnect client
	cl.Disconnect()

	return nil
}
