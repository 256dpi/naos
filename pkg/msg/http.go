package msg

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	subs   sync.Map
}

func (c *httpChannel) Device() Device {
	return c.dev
}

func (c *httpChannel) Subscribe(queue Queue) {
	// add subscription
	c.subs.Store(queue, struct{}{})
}

func (c *httpChannel) Unsubscribe(queue Queue) {
	// remove subscription
	c.subs.Delete(queue)
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
			fmt.Println("httpChannel.reader: read error: " + err.Error())
			return
		}

		// yield message
		for q := range c.subs.Range {
			queue := q.(Queue)
			select {
			case queue <- data:
			default:
				select {
				case queue <- data:
				case <-c.ctx.Done():
					// TODO: Handle error.
					fmt.Println("httpChannel.reader: context done")
					return
				case <-time.After(time.Second):
					// TODO: Handle error.
					fmt.Println("httpChannel.reader: queue full")
					return
				}
			}
		}
	}
}
