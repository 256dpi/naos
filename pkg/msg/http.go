package msg

import (
	"context"
	"fmt"

	"github.com/coder/websocket"
)

type httpDevice struct {
	host string
}

// NewHTTPDevice creates a new HTTP device.
func NewHTTPDevice(host string) Device {
	return &httpDevice{
		host: host,
	}
}

func (d *httpDevice) ID() string {
	return "http/" + d.host
}

func (d *httpDevice) Open() (*Channel, error) {
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
	t := &httpTransport{
		dev:    d,
		ctx:    ctx,
		conn:   conn,
		cancel: cancel,
	}

	// set flag
	ok = true

	return NewChannel(t, d, 10), nil
}

type httpTransport struct {
	dev    *httpDevice
	ctx    context.Context
	conn   *websocket.Conn
	cancel context.CancelFunc
}

func (t *httpTransport) Read() ([]byte, error) {
	// read message
	_, data, err := t.conn.Read(t.ctx)
	return data, err
}

func (t *httpTransport) Write(bytes []byte) error {
	// write message
	return t.conn.Write(t.ctx, websocket.MessageBinary, bytes)
}

func (t *httpTransport) Close() {
	// cancel context
	defer t.cancel()

	// close connection
	_ = t.conn.Close(websocket.StatusNormalClosure, "")
}
