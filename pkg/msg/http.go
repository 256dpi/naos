package msg

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/coder/websocket"
)

// TODO: Move locking to message protocol.

type httpChannel struct {
	ctx    context.Context
	conn   *websocket.Conn
	cancel context.CancelFunc
	subs   map[Queue]struct{}
	mutex  sync.Mutex
}

// OpenHTTPChannel opens a new HTTP channel to the server.
func OpenHTTPChannel(host, password string) (Channel, error) {
	// create context
	var ok bool
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if !ok {
			cancel()
		}
	}()

	// connect to server
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("http://%s:80/naos.sock", host), &websocket.DialOptions{
		Subprotocols: []string{"naos"},
	})
	if err != nil {
		return nil, err
	}

	// prepare channel
	c := &httpChannel{
		ctx:    ctx,
		conn:   conn,
		cancel: cancel,
		subs:   make(map[Queue]struct{}),
	}

	// check lock status
	lockStatus, err := c.rpc("lock", "lock#")
	if err != nil {
		return nil, err
	}

	// check lock status
	if lockStatus == "lock#locked" && password == "" {
		return nil, fmt.Errorf("password required")
	}

	// unlock channel
	if lockStatus == "lock#locked" {
		lockStatus, err = c.rpc("lock#"+password, "lock#")
		if err != nil {
			return nil, err
		} else if lockStatus != "lock#unlocked" {
			return nil, fmt.Errorf("invalid password")
		}
	}

	// set flag
	ok = true

	// run reader
	go c.reader()

	return c, nil
}

func (c *httpChannel) Name() string {
	return "http"
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
	// prefix message
	bytes = append([]byte("msg:"), bytes...)

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

func (c *httpChannel) rpc(msg, filter string) (string, error) {
	// write message
	err := c.conn.Write(c.ctx, websocket.MessageText, []byte(msg))
	if err != nil {
		return "", err
	}

	// read response
	var data []byte
	for {
		_, data, err = c.conn.Read(c.ctx)
		if err != nil {
			return "", err
		}
		if filter == "" || strings.HasPrefix(string(data), filter) {
			break
		}
	}

	return string(data), nil
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

		// check prefix
		if len(data) >= 4 && string(data[:4]) == "msg#" {
			data = data[4:]
		} else {
			continue
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
