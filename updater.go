package nadm

import (
	"fmt"
	"strconv"

	"github.com/gomqtt/client"
	"github.com/gomqtt/packet"
)

// UpdateFirmware will perform a firmware update an block until it is done or an
// error has occurred. If progress is provided it will be called with the bytes
// sent to the device.
func UpdateFirmware(url, baseTopic string, image []byte, progress func(int)) error {
	// prepare channels
	requests := make(chan int)
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

	// subscribe to next chunk topic
	sf, err := cl.Subscribe(baseTopic+"/nadk/update/next", 0)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait()
	if err != nil {
		return err
	}

	// begin update process by sending the size of the image
	_, err = cl.Publish(baseTopic+"/nadk/update/begin", []byte(strconv.Itoa(len(image))), 0, false)
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
		case next = <-requests:
			// continue
		}

		// calculate remaining bytes
		remaining := len(image) - total

		// check if done
		if remaining == 0 {
			// send finish
			_, err := cl.Publish(baseTopic+"/nadk/update/finish", nil, 0, false)
			if err != nil {
				return err
			}

			// disconnect
			err = cl.Disconnect()
			if err != nil {
				return err
			}

			return nil
		}

		// check chunk size
		if next > remaining {
			// prevent overflow
			next = remaining
		} else if next > 4096 {
			// limit to 4096 bytes
			next = 4096
		}

		// write chunk
		_, err := cl.Publish(baseTopic+"/nadk/update/write", image[total:total+next], 0, false)
		if err != nil {
			return err
		}

		// adjust counter
		total += next

		// update progress if available
		if progress != nil {
			progress(total)
		}
	}
}
