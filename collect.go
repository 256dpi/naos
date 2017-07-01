package naos

import (
	"strings"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// An Announcement is returned by Collect.
type Announcement struct {
	ReceivedAt      time.Time
	BaseTopic       string
	DeviceName      string
	DeviceType      string
	FirmwareVersion string
}

// Collect will collect Announcements from devices by connecting to the provided
// MQTT broker and sending the 'collect' command.
//
// Note: Not correctly formatted announcements are ignored.
func Collect(url string, duration time.Duration) ([]*Announcement, error) {
	// prepare channels
	errs := make(chan error)
	anns := make(chan *Announcement)

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
		if len(data) < 4 {
			return
		}

		// add announcement
		anns <- &Announcement{
			ReceivedAt:      time.Now(),
			BaseTopic:       data[3],
			DeviceType:      data[0],
			FirmwareVersion: data[1],
			DeviceName:      data[2],
		}
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = cf.Wait(duration)
	if err != nil {
		return nil, err
	}

	// make sure client gets closed
	defer cl.Close()

	// subscribe to announcement topic
	sf, err := cl.Subscribe("/naos/announcement", 0)
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = sf.Wait(duration)
	if err != nil {
		return nil, err
	}

	// collect all devices
	_, err = cl.Publish("/naos/collect", []byte(""), 0, false)
	if err != nil {
		return nil, err
	}

	// prepare list
	var list []*Announcement

	// set deadline
	deadline := time.After(duration)

	for {
		// wait for error, announcement or deadline
		select {
		case err := <-errs:
			return list, err
		case a := <-anns:
			list = append(list, a)
		case <-deadline:
			goto exit
		}
	}

exit:

	// disconnect client
	cl.Disconnect()

	return list, nil
}
