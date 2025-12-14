package mqtt

import (
	"sync"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/msg"
)

type device struct {
	router    *Router
	baseTopic string
}

// NewDevice creates a new MQTT device using the given router and base topic.
func NewDevice(r *Router, baseTopic string) msg.Device {
	return &device{
		router:    r,
		baseTopic: baseTopic,
	}
}

func (d *device) ID() string {
	return "mqtt/" + d.baseTopic
}

func (d *device) Open() (msg.Channel, error) {
	// prepare topics
	inbox := d.baseTopic + "/naos/inbox"
	outbox := d.baseTopic + "/naos/outbox"

	// prepare channel
	ch := &channel{
		device: d,
		router: d.router,
		inbox:  inbox,
		outbox: outbox,
	}

	// set callback
	callback := func(m *packet.Message, err error) {
		if err != nil {
			return
		}
		for sub := range ch.subs.Range {
			queue := sub.(msg.Queue)
			select {
			case queue <- m.Payload:
			default:
				// drop message if queue is full
			}
		}
	}

	// subscribe
	id, err := d.router.Subscribe(outbox, callback)
	if err != nil {
		return nil, err
	}

	// set handle
	ch.handle = id

	return ch, nil
}

type channel struct {
	device *device
	subs   sync.Map
	router *Router
	inbox  string
	outbox string
	handle uint64
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
	err := c.router.Publish(c.inbox, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (c *channel) Close() {
	// unsubscribe
	_ = c.router.Unsubscribe(c.outbox, c.handle)
}
