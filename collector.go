package nadm

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
	"gopkg.in/tomb.v2"
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

	Devices chan *Device

	mutex sync.Mutex
	tomb  tomb.Tomb
}

// NewCollector creates and returns a new Collector.
func NewCollector(brokerURL string) *Collector {
	return &Collector{
		BrokerURL: brokerURL,
	}
}

// Start will start with the collection process.
func (c *Collector) Start() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Devices = make(chan *Device)

	c.tomb = tomb.Tomb{}
	c.tomb.Go(c.processor)
}

// Stop will stop the collection process.
func (c *Collector) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.tomb.Kill(nil)
	return c.tomb.Wait()
}

func (c *Collector) processor() error {
	// prepare channels
	errs := make(chan error)

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
		c.Devices <- &Device{
			Type:      data[0],
			Name:      data[1],
			BaseTopic: data[2],
		}
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(c.BrokerURL))
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

	// subscribe to announcement topic
	sf, err := cl.Subscribe("/nadk/announcement", 0)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait()
	if err != nil {
		return err
	}

	// collect all devices
	_, err = cl.Publish("/nadk/collect", []byte(""), 0, false)
	if err != nil {
		return err
	}

	for {
		// wait for error or kill
		select {
		case err := <-errs:
			close(c.Devices)
			return err
		case <-c.tomb.Dying():
			return tomb.ErrDying
		}
	}
}
