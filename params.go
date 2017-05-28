package nadm

import "github.com/gomqtt/client"

// SetParam will publish the provided parameter for all specified base topics.
func SetParam(url, param, value string, baseTopics []string) error {
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
		pf, err := cl.Publish(baseTopic + "/nadk/set/" + param, []byte(value), 0, false)
		if err != nil {
			return err
		}

		// wait for ack
		err = pf.Wait()
		if err != nil {
			return err
		}
	}

	// attempt to disconnect
	_err := cl.Disconnect()
	if err == nil && _err != nil {
		return _err
	}

	return err
}
