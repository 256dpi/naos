package mqtt

import (
	"errors"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// Debug will request coredump debug information from the specified devices.
func Debug(url string, baseTopics []string, delete bool, duration time.Duration) (map[string][]byte, error) {
	// check base topics
	if len(baseTopics) == 0 {
		return nil, errors.New("zero base topics")
	}

	// prepare table
	table := make(map[string][]byte)

	// prepare channels
	errs := make(chan error)

	// fill table
	for _, baseTopic := range baseTopics {
		// create status
		table[baseTopic] = []byte{}
	}

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) {
		// send errors
		if err != nil {
			errs <- err
			return
		}

		// update table
		for _, baseTopic := range baseTopics {
			if msg.Topic == baseTopic+"/naos/coredump" {
				// update coredump
				table[baseTopic] = append(table[baseTopic], msg.Payload...)
			}
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

	// prepare subscriptions
	var subs []packet.Subscription

	// add subscriptions
	for _, baseTopic := range baseTopics {
		subs = append(subs, packet.Subscription{
			Topic: baseTopic + "/naos/coredump",
			QOS:   0,
		})
	}

	// subscribe to next chunk topic
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = sf.Wait(duration)
	if err != nil {
		return nil, err
	}

	// request coredump data
	for _, baseTopic := range baseTopics {
		// prepare payload
		payload := ""
		if delete {
			payload = "delete"
		}

		// publish config update
		pf, err := cl.Publish(baseTopic+"/naos/debug", []byte(payload), 0, false)
		if err != nil {
			return nil, err
		}

		// wait for ack
		err = pf.Wait(duration)
		if err != nil {
			return nil, err
		}
	}

	// wait for error or duration
	select {
	case err = <-errs:
		return nil, err
	case <-time.After(duration):
		cl.Disconnect()
		return table, nil
	}
}
