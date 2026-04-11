package mqtt

import (
	"io"
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

func (d *device) Type() string {
	return "MQTT"
}

func (d *device) Name() string {
	return d.baseTopic
}

func (d *device) Open() (*msg.Channel, error) {
	// prepare topics
	inbox := d.baseTopic + "/naos/inbox"
	outbox := d.baseTopic + "/naos/outbox"

	// prepare transport
	ch := &transport{
		device: d,
		reads:  make(chan []byte, 64),
		done:   make(chan struct{}),
		router: d.router,
		inbox:  inbox,
		outbox: outbox,
	}

	// subscribe outbox
	id, err := d.router.Subscribe(outbox, func(m *packet.Message, err error) {
		if err != nil {
			return
		}
		select {
		case ch.reads <- append([]byte(nil), m.Payload...):
		default:
			ch.Close()
		}
	})
	if err != nil {
		return nil, err
	}

	// set handle
	ch.handle = id

	return msg.NewChannel(ch, d, 10), nil
}

type transport struct {
	device *device
	reads  chan []byte
	done   chan struct{}
	router *Router
	inbox  string
	outbox string
	handle uint64
	mutex  sync.Mutex
	once   sync.Once
}

func (t *transport) Read() ([]byte, error) {
	// read message
	select {
	case data := <-t.reads:
		return data, nil
	case <-t.done:
		return nil, io.EOF
	}
}

func (t *transport) Write(bytes []byte) error {
	// send message
	err := t.router.Publish(t.inbox, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (t *transport) Close() {
	// close transport
	t.once.Do(func() {
		close(t.done)
		t.mutex.Lock()
		_ = t.router.Unsubscribe(t.outbox, t.handle)
		t.mutex.Unlock()
	})
}
