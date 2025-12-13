package mqtt

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// The Description of a MQTT device available.
type Description struct {
	AppName    string
	AppVersion string
	DeviceName string
	BaseTopic  string
}

// Discover connects to the given URI and continuously yield discovered device
// descriptions. The path of the URI is used as the base topic to construct the
// describe and discover topics.
func Discover(ctx context.Context, uri string, qos int, handle func(Description)) error {
	// wrap context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// check QOS
	pktQOS := packet.QOS(qos)
	if !pktQOS.Successful() {
		panic("invalid QOS")
	}

	// create client
	cl := client.New()

	// connect client
	cf, err := cl.Connect(client.NewConfig(uri))
	if err != nil {
		return err
	}
	err = cf.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	// assign callback
	cl.Callback = func(m *packet.Message, err error) error {
		// check for error or empty message
		if err != nil {
			cancel()
			return err
		} else if len(m.Payload) == 0 {
			return nil
		}

		// pluck of version
		version, rest, ok := strings.Cut(string(m.Payload), "|")
		if !ok || version != "0" {
			return nil
		}

		// parse rest
		fields := strings.Split(rest, "|")
		if len(fields) != 4 {
			return nil
		}

		// handle device
		handle(Description{
			AppName:    fields[0],
			AppVersion: fields[1],
			DeviceName: fields[2],
			BaseTopic:  fields[3],
		})

		return nil
	}

	// prepare discover topic
	baseTopic := urlPath(uri)
	discoverTopic := baseTopic + "/naos/discover"
	describeTopic := baseTopic + "/naos/describe"

	// subscribe to describe topic
	sf, err := cl.Subscribe(describeTopic, pktQOS)
	if err != nil {
		return err
	}
	err = sf.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	// trigger discovery
	pf, err := cl.Publish(discoverTopic, []byte{}, pktQOS, false)
	if err != nil {
		return err
	}
	err = pf.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	// wait for context cancellation
	<-ctx.Done()

	// disconnect client
	_ = cl.Disconnect()

	return ctx.Err()
}

// Collect connects to the given URI and collects discovered device descriptions
// into a slice which is returned once the context is cancelled.
func Collect(ctx context.Context, uri string, qos int) ([]Description, error) {
	// collect devices
	var list []Description
	err := Discover(ctx, uri, qos, func(d Description) {
		list = append(list, d)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}

	return list, nil
}
