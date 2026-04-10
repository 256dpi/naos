package connect

import (
	"net"
	"sync"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

const (
	version = 0x1
	cmdMsg  = 0x0
)

// Conn exchanges packet-oriented binary messages over a connection.
type Conn interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetReadDeadline(time.Time) error
	Read() ([]byte, error)
	Write([]byte) (int, error)
	Close() error
}

// Channel adapts a connection to the msg.Channel interface.
type Channel struct {
	dev  msg.Device
	conn Conn
	subs sync.Map
	done chan struct{}
	once sync.Once
}

// NewChannel wraps the provided connection in a messaging channel.
func NewChannel(dev msg.Device, conn Conn) *Channel {
	// create channel
	ch := &Channel{
		dev:  dev,
		conn: conn,
		done: make(chan struct{}),
	}

	// run reader
	go ch.reader()

	return ch
}

// Width returns the maximum number of concurrent sessions.
func (c *Channel) Width() int {
	return 10
}

// Device returns the underlying device if available.
func (c *Channel) Device() msg.Device {
	return c.dev
}

// Subscribe registers a queue to receive incoming messages.
func (c *Channel) Subscribe(queue msg.Queue) {
	c.subs.Store(queue, struct{}{})
}

// Unsubscribe removes a queue from the subscriber list.
func (c *Channel) Unsubscribe(queue msg.Queue) {
	c.subs.Delete(queue)
}

// Write sends a single framed message.
func (c *Channel) Write(data []byte) error {
	// prepare frame
	buf := make([]byte, 2+len(data))
	buf[0] = version
	buf[1] = cmdMsg
	copy(buf[2:], data)

	// send frame
	_, err := c.conn.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the channel and the underlying connection.
func (c *Channel) Close() {
	c.closeDone()
	_ = c.conn.Close()
}

// Done is closed when the reader stops and the channel is no longer usable.
func (c *Channel) Done() <-chan struct{} {
	return c.done
}

func (c *Channel) closeDone() {
	c.once.Do(func() {
		close(c.done)
	})
}

func (c *Channel) reader() {
	// ensure closing
	defer c.closeDone()

	for {
		// read frame
		buf, err := c.conn.Read()
		if err != nil {
			return
		}

		// check header
		if len(buf) < 2 || buf[0] != version || buf[1] != cmdMsg {
			_ = c.conn.Close()
			return
		}

		// copy data
		data := append([]byte(nil), buf[2:]...)

		// distribute data
		c.subs.Range(func(key, _ any) bool {
			queue := key.(msg.Queue)
			select {
			case queue <- data:
			default:
				select {
				case queue <- data:
				case <-time.After(time.Second):
				case <-c.done:
				}
			}
			return true
		})
	}
}
