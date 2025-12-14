package mqtt

import (
	"context"
	"errors"
	"strings"

	"github.com/256dpi/gomqtt/packet"
)

// The Description of a MQTT device available.
type Description struct {
	AppName    string
	AppVersion string
	DeviceName string
	BaseTopic  string
}

// Discover uses the provided Router to discover connected MQTT devices.
func Discover(ctx context.Context, r *Router, handle func(Description)) error {
	// wrap context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// prepare callback
	callback := func(m *packet.Message, err error) {
		// check for error or empty message
		if err != nil {
			cancel()
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

	// trigger discovery
	err = r.Publish(discoverTopic, []byte{})
	if err != nil {
		return err
	}

	// wait for context cancellation
	<-ctx.Done()

	return ctx.Err()
}

// Collect uses the provided Router to collect connected MQTT devices.
func Collect(ctx context.Context, r *Router) ([]Description, error) {
	// collect devices
	var list []Description
	err := Discover(ctx, r, func(d Description) {
		list = append(list, d)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}

	return list, nil
}
