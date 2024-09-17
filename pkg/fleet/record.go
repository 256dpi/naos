package fleet

import (
	"errors"
	"strings"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// LogMessage is emitted by Record.
type LogMessage struct {
	Time      time.Time
	BaseTopic string
	Content   string
}

// Record will enable log recording mode and yield the received log messages
// until the provided channel has been closed.
func Record(url string, baseTopics []string, quit chan struct{}, timeout time.Duration, cb func(*LogMessage)) error {
	// check base topics
	if len(baseTopics) == 0 {
		return errors.New("zero base topics")
	}

	// prepare channels
	errs := make(chan error)

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) error {
		// send errors
		if err != nil {
			errs <- err
			return nil
		}

		// prepare log message
		log := &LogMessage{
			Time:    time.Now(),
			Content: string(msg.Payload),
		}

		// set base topic
		for _, baseTopic := range baseTopics {
			if strings.HasPrefix(msg.Topic, baseTopic) {
				log.BaseTopic = baseTopic
			}
		}

		// call callback
		cb(log)

		return nil
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return err
	}

	// wait for ack
	err = cf.Wait(timeout)
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
			Topic: baseTopic + "/naos/log",
			QOS:   0,
		})
	}

	// subscribe to next chunk topic
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait(timeout)
	if err != nil {
		return err
	}

	// enable message recording
	for _, baseTopic := range baseTopics {
		// publish config update
		pf, err := cl.Publish(baseTopic+"/naos/record/", []byte("on"), 0, false)
		if err != nil {
			return err
		}

		// wait for ack
		err = pf.Wait(timeout)
		if err != nil {
			return err
		}
	}

	// wait for error or quit
	select {
	case err = <-errs:
		return err
	case <-quit:
		// move on
	}

	// disable message recording
	for _, baseTopic := range baseTopics {
		// publish config update
		pf, err := cl.Publish(baseTopic+"/naos/record/", []byte("off"), 0, false)
		if err != nil {
			return err
		}

		// wait for ack
		err = pf.Wait(timeout)
		if err != nil {
			return err
		}
	}

	// disconnect client
	cl.Disconnect()

	return nil
}
