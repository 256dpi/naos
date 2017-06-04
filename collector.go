package nadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// An Announcement is returned by Collect.
type Announcement struct {
	DeviceType      string
	FirmwareVersion string
	DeviceName      string
	BaseTopic       string
}

// Collect will collect Announcements.
func Collect(url string, d time.Duration) ([]*Announcement, error) {
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
			errs <- fmt.Errorf("malformed payload: %s", string(msg.Payload))
			return
		}

		// add announcement
		anns <- &Announcement{
			DeviceType:      data[0],
			FirmwareVersion: data[1],
			DeviceName:      data[2],
			BaseTopic:       data[3],
		}
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = cf.Wait()
	if err != nil {
		return nil, err
	}

	// make sure client gets closed
	defer cl.Close()

	// subscribe to announcement topic
	sf, err := cl.Subscribe("/nadk/announcement", 0)
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = sf.Wait()
	if err != nil {
		return nil, err
	}

	// collect all devices
	_, err = cl.Publish("/nadk/collect", []byte(""), 0, false)
	if err != nil {
		return nil, err
	}

	// prepare list
	var list []*Announcement

	// set deadline
	deadline := time.After(d)

	for {
		// wait for error, announcement or deadline
		select {
		case err := <-errs:
			return list, err
		case a := <-anns:
			list = append(list, a)
		case <-deadline:
			// disconnect
			err = cl.Disconnect()
			if err != nil {
				return list, err
			}

			return list, nil
		}
	}
}
