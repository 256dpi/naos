package nadm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// An Updater is responsible for updating a single device over MQTT to a new
// image.
type Updater struct {
	BrokerURL string
	BaseTopic string
	Image     []byte

	OnProgress func(sent int)
}

// NewUpdater creates and returns a new Updater.
func NewUpdater(brokerURL, baseTopic string, image []byte) *Updater {
	return &Updater{
		BrokerURL: brokerURL,
		BaseTopic: baseTopic,
		Image:     image,
	}
}

// TODO: Base Topic should never end in a slash.

// TODO: Use tomb, and allow cancel and timeout.

// Run will perform the update an block until it is done or an error has
// occurred.
func (u *Updater) Run() error {
	// prepare channels
	requests := make(chan int)
	errs := make(chan error)
	finished := make(chan struct{})

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) {
		// send errors
		if err != nil {
			errs <- err
			return
		}

		// handle finished message
		if strings.HasSuffix(msg.Topic, "nadk/ota/finished") {
			close(finished)
			return
		}

		// otherwise convert the chunk request
		n, err := strconv.ParseInt(string(msg.Payload), 10, 0)
		if err != nil {
			errs <- err
			return
		}

		// check size
		if n <= 0 {
			errs <- fmt.Errorf("invalid chunk request of size %d", n)
			return
		}

		// send chunk request
		requests <- int(n)
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(u.BrokerURL))
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

	// subscribe to next chunk topic
	sf, err := cl.Subscribe(u.BaseTopic+"/nadk/ota/next", 0)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait()
	if err != nil {
		return err
	}

	// subscribe to finished topic
	sf, err = cl.Subscribe(u.BaseTopic+"/nadk/ota/finished", 0)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait()
	if err != nil {
		return err
	}

	// begin update process by sending the size of the image
	_, err = cl.Publish(u.BaseTopic+"/nadk/ota", []byte(strconv.Itoa(len(u.Image))), 0, false)
	if err != nil {
		return err
	}

	// prepare counters
	total := 0

	for {
		// the size of the next chunk
		next := 0

		// wait for error or request
		select {
		case err := <-errs:
			return err
		case <-finished:
			return nil
		case next = <-requests:
			// continue
		}

		// calculate remaining bytes
		remaining := len(u.Image) - total

		// prevent overflow
		if next > remaining {
			next = remaining
		}

		// send chunk
		_, err := cl.Publish(u.BaseTopic+"/nadk/ota/chunk", u.Image[total:total+next], 0, false)
		if err != nil {
			return err
		}

		// adjust counter
		total += next

		// update progress if available
		if u.OnProgress != nil {
			u.OnProgress(total)
		}
	}
}
