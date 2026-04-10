package http

import (
	"context"
	"fmt"

	"github.com/coder/websocket"

	"github.com/256dpi/naos/pkg/msg"
)

type device struct {
	host string
}

// NewDevice creates a new HTTP device.
func NewDevice(host string) msg.Device {
	return &device{
		host: host,
	}
}

func (d *device) ID() string {
	return "http/" + d.host
}

func (d *device) Open() (*msg.Channel, error) {
	// create context
	var ok bool
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if !ok {
			cancel()
		}
	}()

	// connect to server
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("http://%s:80", d.host), &websocket.DialOptions{
		Subprotocols: []string{"naos"},
	})
	if err != nil {
		return nil, err
	}

	// prepare transport
	t := &transport{
		dev:    d,
		ctx:    ctx,
		conn:   conn,
		cancel: cancel,
	}

	// set flag
	ok = true

	return msg.NewChannel(t, d, 10), nil
}

type transport struct {
	dev    *device
	ctx    context.Context
	conn   *websocket.Conn
	cancel context.CancelFunc
}

func (t *transport) Read() ([]byte, error) {
	// read message
	_, data, err := t.conn.Read(t.ctx)
	return data, err
}

func (t *transport) Write(bytes []byte) error {
	// write message
	return t.conn.Write(t.ctx, websocket.MessageBinary, bytes)
}

func (t *transport) Close() {
	// cancel context
	defer t.cancel()

	// close connection
	_ = t.conn.Close(websocket.StatusNormalClosure, "")
}
