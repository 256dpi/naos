package nadm

import (
	"strings"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// Set will publish the provided parameter for all specified base topics.
func Set(url, param, value string, baseTopics []string, d time.Duration) (map[string]string, error) {
	return commonGetSet(url, param, value, true, baseTopics, d)
}

// Get will publish the provided parameter for all specified base topics.
func Get(url, param string, baseTopics []string, d time.Duration) (map[string]string, error) {
	return commonGetSet(url, param, "", false, baseTopics, d)
}

func commonGetSet(url, param, value string, set bool, baseTopics []string, d time.Duration) (map[string]string, error) {
	// prepare channels
	errs := make(chan error)
	response := make(chan struct{})

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
			if strings.HasPrefix(msg.Topic, baseTopic) {
				table[baseTopic] = strings.TrimPrefix(msg.Topic, baseTopic+"/nadk/value/")
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
		// init variables
		topic := baseTopic + "/nadk/get/" + param
		payload := ""

		// override if set is set
		if set {
			topic = baseTopic + "/nadk/set/" + param
			payload = value
		}

		// publish config update
		pf, err := cl.Publish(topic, []byte(payload), 0, false)
		if err != nil {
			return nil, err
		}

		// wait for ack
		err = pf.Wait()
		if err != nil {
			return nil, err
		}
	}

	// prepare counter
	counter := len(baseTopics)

	// wait for errors, counter or timeout
	for {
		select {
		case err = <-errs:
			return table, err
		case <-response:
			counter--

			if counter == 0 {
				goto exit
			}
		case <-time.After(d):
			goto exit
		}
	}

exit:

	// disconnect client
	cl.Disconnect()

	return table, nil
}
