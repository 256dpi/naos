package mqtt

import (
	"sync"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/msg"
)

// TODO: Support multiple devices per MQTT client?

type device struct {
	url string
	qos packet.QOS
}

// NewDevice creates a new MQTT device for the given broker URL and QOS level.
// The path part of the URL will be used as the device base topic to construct
// inbox and outbox topics.
func NewDevice(url string, qos int) msg.Device {
	// check QOS
	pktQOS := packet.QOS(qos)
	if !pktQOS.Successful() {
		panic("invalid QOS")
	}

	return &device{
		url: url,
		qos: pktQOS,
	}
}

func (d *device) ID() string {
	return "mqtt/" + d.url
}

func (d *device) Open() (msg.Channel, error) {
	// prepare topics
	baseTopic := urlPath(d.url)
	inbox := baseTopic + "/naos/inbox"
	outbox := baseTopic + "/naos/outbox"

	// create client
	cl := client.New()

	// connect client
	cf, err := cl.Connect(client.NewConfig(d.url))
	if err != nil {
		return nil, err
	}
	err = cf.Wait(5 * time.Second)
	if err != nil {
		return nil, err
	}

	// prepare channel
	ch := &channel{
		device: d,
		client: cl,
		inbox:  inbox,
		qos:    d.qos,
	}

	// set callback
	cl.Callback = func(m *packet.Message, err error) error {
		if err != nil {
			return err
		}
		for sub := range ch.subs.Range {
			queue := sub.(msg.Queue)
			select {
			case queue <- m.Payload:
			default:
				// drop message if queue is full
			}
		}
		return nil
	}

	// subscribe
	sf, err := cl.Subscribe(outbox, d.qos)
	if err != nil {
		return nil, err
	}
	err = sf.Wait(5 * time.Second)
	if err != nil {
		return nil, err
	}

	return ch, nil
}

type channel struct {
	device *device
	subs   sync.Map
	client *client.Client
	inbox  string
	qos    packet.QOS
}

func (c *channel) Width() int {
	return 10
}

func (c *channel) Device() msg.Device {
	return c.device
}

func (c *channel) Subscribe(queue msg.Queue) {
	// add queue
	c.subs.Store(queue, nil)
}

func (c *channel) Unsubscribe(queue msg.Queue) {
	// remove queue
	c.subs.Delete(queue)
}

func (c *channel) Write(bytes []byte) error {
	// send message
	future, err := c.client.Publish(c.inbox, bytes, c.qos, false)
	if err != nil {
		return err
	}

	// await future
	err = future.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	return nil
}

func (c *channel) Close() {
	// disconnect
	_ = c.client.Disconnect()
}
