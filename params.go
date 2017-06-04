package nadm

import (
	"strings"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// Set will publish the provided parameter for all specified base topics.
func Set(url, param, value string, baseTopics []string) error {
	// create client
	cl := client.New()

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return err
	}

	// wait for ack
	err = cf.Wait()
	if err != nil {
		return err
	}

	// make sure client gets closed
	defer cl.Close()

	// add subscriptions
	for _, baseTopic := range baseTopics {
		// publish config update
		pf, err := cl.Publish(baseTopic+"/nadk/set/"+param, []byte(value), 0, false)
		if err != nil {
			return err
		}

		// wait for ack
		err = pf.Wait()
		if err != nil {
			return err
		}
	}

	// disconnect client
	err = cl.Disconnect()
	if err != nil {
		return err
	}

	return nil
}

// Get will publish the provided parameter for all specified base topics.
func Get(url, param string, baseTopics []string, d time.Duration) (map[string]string, error) {
	// prepare errors channel
	errs := make(chan error)

	// prepare response
	table := make(map[string]string)

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
			if strings.HasPrefix(msg.Topic, baseTopic) {
				table[baseTopic] = strings.TrimPrefix(msg.Topic, baseTopic+"/nadk/value/")
			}
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

	// prepare subscriptions
	var subs []packet.Subscription

	// add subscriptions
	for _, baseTopic := range baseTopics {
		subs = append(subs, packet.Subscription{
			Topic: baseTopic + "/nadk/value/+",
			QOS:   0,
		})
	}

	// subscribe to next chunk topic
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = sf.Wait()
	if err != nil {
		return nil, err
	}

	// add subscriptions
	for _, baseTopic := range baseTopics {
		// publish config update
		pf, err := cl.Publish(baseTopic+"/nadk/get/"+param, nil, 0, false)
		if err != nil {
			return nil, err
		}

		// wait for ack
		err = pf.Wait()
		if err != nil {
			return nil, err
		}
	}

	// wait for errors or timeout
	select {
	case err = <-errs:
		return nil, err
	case <-time.After(d):
		// move on
	}

	// disconnect client
	err = cl.Disconnect()
	if err != nil {
		return nil, err
	}

	return table, nil
}
