package mqtt

import (
	"errors"
	"strings"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// Discover will connect to the specified MQTT broker and publish the 'discover'
// command to receive a list of available parameters.
func Discover(url string, baseTopics []string, timeout time.Duration) (map[string][]string, error) {
	// check base topics
	if len(baseTopics) == 0 {
		return nil, errors.New("zero base topics")
	}

	// prepare channels
	errs := make(chan error, 1)
	response := make(chan struct{}, len(baseTopics))

	// prepare table
	table := make(map[string][]string)

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) error {
		// send errors
		if err != nil {
			errs <- err
			return nil
		}

		// parse message
		segments := strings.Split(string(msg.Payload), ",")

		// create parameters
		list := make([]string, 0, len(segments))
		for _, s := range segments {
			subSegments := strings.Split(s, ":")
			list = append(list, subSegments[0])
		}

		// update table
		for _, baseTopic := range baseTopics {
			if msg.Topic == baseTopic+"/naos/parameters" {
				table[baseTopic] = list
				response <- struct{}{}
			}
		}

		return nil
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
			Topic: baseTopic + "/naos/parameters",
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
		// publish discover
		pf, err := cl.Publish(baseTopic+"/naos/discover", nil, 0, false)
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
	// check base topics
	if len(baseTopics) == 0 {
		return errors.New("zero base topics")
	}

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
	// check base topics
	if len(baseTopics) == 0 {
		return nil, errors.New("zero base topics")
	}

	// prepare channels
	errs := make(chan error, 1)
	response := make(chan struct{}, len(baseTopics))

	// prepare table
	table := make(map[string]string)

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) error {
		// send errors
		if err != nil {
			errs <- err
			return nil
		}

		// update table
		for _, baseTopic := range baseTopics {
			if msg.Topic == baseTopic+"/naos/value/"+param {
				table[baseTopic] = string(msg.Payload)
				response <- struct{}{}
			}
		}

		return nil
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
