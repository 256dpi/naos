package nadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// A Device is returned by the collector.
type Device struct {
	Type      string
	Name      string
	BaseTopic string
}

// A Collector collects NADK devices connected to a broker.
type Collector struct {
	BrokerURL  string
	BaseTopics []string
}

// NewCollector creates and returns a new Collector.
func NewCollector(brokerURL string) *Collector {
	return &Collector{
		BrokerURL: brokerURL,
	}
}

// Run will collector devices and block for the specified duration.
func (c *Collector) Run(timeout time.Duration) ([]Device, error) {
	// prepare channels
	errs := make(chan error)
	devices := make(chan Device)

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) {
		// send errors
		if err != nil {
			errs <- err
			return
		}

		// TODO: Make sure nobody uses a comma.

		// get data from payload
		data := strings.Split(string(msg.Payload), ",")

		// check length
		if len(data) < 3 {
			errs <- fmt.Errorf("malformed payload: %s", string(msg.Payload))
			return
		}

		// add device
		devices <- Device{
			Type:      data[0],
			Name:      data[1],
			BaseTopic: data[2],
		}
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(c.BrokerURL))
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

	// get deadline
	deadline := time.After(timeout)

	// prepare list
	var list []Device

	for {
		// wait for error, device or deadline
		select {
		case err := <-errs:
			return nil, err
		case dev := <-devices:
			list = append(list, dev)
		case <-deadline:
			return list, nil
		}
	}
}
