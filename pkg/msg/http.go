package msg

import (
	"context"
	"fmt"
	"sync"

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

func (d *httpDevice) Open() (Channel, error) {
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

	// prepare channel
	c := &httpChannel{
		dev:    d,
		ctx:    ctx,
		conn:   conn,
		cancel: cancel,
		subs:   make(map[Queue]struct{}),
	}

	// set flag
	ok = true

	// run reader
	go c.reader()

	return c, nil
}

type httpChannel struct {
	dev    *httpDevice
	ctx    context.Context
	conn   *websocket.Conn
	cancel context.CancelFunc
	subs   map[Queue]struct{}
	mutex  sync.Mutex
}

func (c *httpChannel) Device() Device {
	return c.dev
}

func (c *httpChannel) Subscribe(ch Queue) {
	// acquire mutex
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// add subscription
	c.subs[ch] = struct{}{}
}

func (c *httpChannel) Unsubscribe(ch Queue) {
	// acquire mutex
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// remove subscription
	delete(c.subs, ch)
}

func (c *httpChannel) Write(bytes []byte) error {
	// write message
	err := c.conn.Write(c.ctx, websocket.MessageBinary, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (c *httpChannel) Close() {
	// cancel context
	defer c.cancel()

	// close connection
	_ = c.conn.Close(websocket.StatusNormalClosure, "")
}

func (c *httpChannel) reader() {
	for {
		// read messages
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			// TODO: Handle error.
			fmt.Println(err)
			return
		}

		// yield message
		c.mutex.Lock()
		for ch := range c.subs {
			select {
			case ch <- data:
			default:
			}
		}
		c.mutex.Unlock()
	}
}
