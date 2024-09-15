package fleet

import (
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// Receive will receive a message from all provided topics.
func Receive(url string, topics []string, timeout time.Duration) (map[string]string, error) {
	// create client
	cl := client.New()

	// set callback
	done := make(chan struct{})
	store := make(map[string]string)
	cl.Callback = func(msg *packet.Message, err error) error {
		// handle error
		if err != nil {
			return err
		}

		// queue message
		store[msg.Topic] = string(msg.Payload)

		// check if all messages have been received
		if len(store) == len(topics) {
			close(done)
		}

		return nil
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return nil, err
	}
	err = cf.Wait(timeout)
	if err != nil {
		return nil, err
	}

	// make sure client gets closed
	defer cl.Close()

	// subscribe topics
	var subs []packet.Subscription
	for _, topic := range topics {
		subs = append(subs, packet.Subscription{
			Topic: topic,
			QOS:   0,
		})
	}
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return nil, err
	}
	err = sf.Wait(timeout)
	if err != nil {
		return nil, err
	}

	// wait for all messages to be received
	select {
	case <-done:
	case <-time.After(timeout):
	}

	// disconnect client
	cl.Disconnect()

	return store, nil
}
