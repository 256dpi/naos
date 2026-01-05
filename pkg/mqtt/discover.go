package mqtt

import (
	"strings"
	"time"

	"github.com/256dpi/gomqtt/packet"
)

// The Description of a MQTT device.
type Description struct {
	AppName    string
	AppVersion string
	DeviceName string
	BaseTopic  string
}

// Discover uses the provided Router to discover connected MQTT devices.
func Discover(r *Router, stop chan struct{}, handle func(Description)) error {
	// prepare callback
	callback := func(m *packet.Message, err error) {
		// check for error or empty message
		if err != nil {
			return
		} else if len(m.Payload) == 0 {
			return
		}

		// pluck of version
		version, rest, ok := strings.Cut(string(m.Payload), "|")
		if !ok || version != "0" {
			return
		}

		// parse rest
		fields := strings.Split(rest, "|")
		if len(fields) != 4 {
			return
		}

		// handle device
		go handle(Description{
			AppName:    fields[0],
			AppVersion: fields[1],
			DeviceName: fields[2],
			BaseTopic:  fields[3],
		})
	}

	// prepare topics
	discoverTopic := "/naos/discover"
	describeTopic := "/naos/describe"

	// subscribe to describe topic
	id, err := r.Subscribe(describeTopic, callback)
	if err != nil {
		return err
	}

	// ensure unsubscribe
	defer func() {
		_ = r.Unsubscribe(describeTopic, id)
	}()

	for {
		// publish discover message
		err = r.Publish(discoverTopic, []byte{})
		if err != nil {
			return err
		}

		// wait for context cancellation
		select {
		case <-stop:
			return nil
		case <-time.After(5 * time.Second):
			// continue
		}
	}
}

// Collect uses the provided Router to collect connected MQTT devices.
func Collect(r *Router, timeout time.Duration) ([]Description, error) {
	// create context
	stop := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		close(stop)
	}()

	// collect devices
	var list []Description
	err := Discover(r, stop, func(d Description) {
		list = append(list, d)
	})

	return list, err
}
