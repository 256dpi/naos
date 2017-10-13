package mqtt

import (
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// GetParams will connect to the specified MQTT broker and publish the 'get'
// command to receive the provided parameter for all specified base topics.
func GetParams(url, param string, baseTopics []string, timeout time.Duration) (map[string]string, error) {
	return commonGetSet(url, param, "", false, baseTopics, timeout)
}

// SetParams will connect to the specified MQTT broker and publish the 'set'
// command to receive the provided updated parameter for all specified base topics.
func SetParams(url, param, value string, baseTopics []string, timeout time.Duration) (map[string]string, error) {
	return commonGetSet(url, param, value, true, baseTopics, timeout)
}

// UnsetParams will connect to the specified MQTT broker and publish the 'unset'
// command to unset the provided parameter for all specified base topics.
func UnsetParams(url, param string, baseTopics []string, timeout time.Duration) error {
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

	// send unset commands
	for _, baseTopic := range baseTopics {
		// init variables
		topic := baseTopic + "/naos/unset/" + param

		// publish config update
		pf, err := cl.Publish(topic, nil, 0, false)
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
	err = cl.Disconnect()
	if err != nil {
		return err
	}

	return nil
}

func commonGetSet(url, param, value string, set bool, baseTopics []string, timeout time.Duration) (map[string]string, error) {
	// prepare channels
	errs := make(chan error, 1)
	response := make(chan struct{}, len(baseTopics))

	// prepare table
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
			if msg.Topic == baseTopic+"/naos/value/"+param {
				table[baseTopic] = string(msg.Payload)
				response <- struct{}{}
			}
		}
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = cf.Wait(timeout)
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
			Topic: baseTopic + "/naos/value/" + param,
			QOS:   0,
		})
	}

	// subscribe to next chunk topic
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return nil, err
	}

	// wait for ack
	err = sf.Wait(timeout)
	if err != nil {
		return nil, err
	}

	// send get or set commands
	for _, baseTopic := range baseTopics {
		// init variables
		topic := baseTopic + "/naos/get/" + param
		payload := ""

		// override if set is set
		if set {
			topic = baseTopic + "/naos/set/" + param
			payload = value
		}

		// publish config update
		pf, err := cl.Publish(topic, []byte(payload), 0, false)
		if err != nil {
			return nil, err
		}

		// wait for ack
		err = pf.Wait(timeout)
		if err != nil {
			return nil, err
		}
	}

	// prepare counter
	counter := len(baseTopics)

	// prepare timeout
	deadline := time.After(timeout)

	// wait for errors, counter or timeout
	for {
		select {
		case err = <-errs:
			return table, err
		case <-response:
			if counter--; counter == 0 {
				return table, cl.Disconnect()
			}
		case <-deadline:
			return table, cl.Disconnect()
		}
	}
}
