package fleet

import (
	"time"

	"github.com/256dpi/gomqtt/client"
)

// Send will send a message to all provided topics.
func Send(url string, topics []string, message string, timeout time.Duration) error {
	// create client
	cl := client.New()

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

	// publish all messages
	for _, topic := range topics {
		_, err = cl.Publish(topic, []byte(message), 0, false)
		if err != nil {
			return err
		}
	}

	// disconnect client
	cl.Disconnect()

	return nil
}
