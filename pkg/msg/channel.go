package msg

import (
	"sync"
	"time"
)

// Transport exchanges raw messages with a device or peer.
type Transport interface {
	Read() ([]byte, error)
	Write([]byte) error
	Close()
}

// Channel routes raw transport traffic to subscribers based on session ownership.
type Channel struct {
	tr       Transport
	dev      Device
	width    int
	done     chan struct{}
	once     sync.Once
	mu       sync.Mutex
	queues   map[Queue]struct{}
	opening  map[string]Queue
	sessions map[uint16]Queue
	closing  map[uint16]Queue
}

const dispatchTimeout = time.Second

// NewChannel wraps the provided transport in a session-aware channel.
func NewChannel(tr Transport, dev Device, width int) *Channel {
	ch := &Channel{
		tr:       tr,
		dev:      dev,
		width:    width,
		done:     make(chan struct{}),
		queues:   make(map[Queue]struct{}),
		opening:  make(map[string]Queue),
		sessions: make(map[uint16]Queue),
		closing:  make(map[uint16]Queue),
	}

	go ch.reader()

	return ch
}

// Width returns the maximum number of inflight messages.
func (c *Channel) Width() int {
	return c.width
}

// Device returns the underlying device if available.
func (c *Channel) Device() Device {
	return c.dev
}

// Subscribe registers a queue to receive incoming messages.
func (c *Channel) Subscribe(queue Queue) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// add queue
	c.queues[queue] = struct{}{}
}

// Unsubscribe removes a queue from the subscriber list.
func (c *Channel) Unsubscribe(queue Queue) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// remove queue
	delete(c.queues, queue)
	for handle, owner := range c.opening {
		if owner == queue {
			delete(c.opening, handle)
		}
	}
	for session, owner := range c.sessions {
		if owner == queue {
			delete(c.sessions, session)
			delete(c.closing, session)
		}
	}
	for session, owner := range c.closing {
		if owner == queue {
			delete(c.closing, session)
		}
	}
}

// Write sends a single framed message on behalf of a specific queue. A nil
// queue means the write has no subscriber ownership context.
func (c *Channel) Write(queue Queue, msg Message) error {
	return c.write(queue, msg)
}

// Close closes the channel and the underlying transport.
func (c *Channel) Close() {
	c.closeDone()
	c.tr.Close()
}

// Done is closed when the underlying transport reader exits.
func (c *Channel) Done() <-chan struct{} {
	return c.done
}

func (c *Channel) closeDone() {
	c.once.Do(func() {
		close(c.done)
	})
}

func (c *Channel) reader() {
	defer c.closeDone()

	for {
		// read data
		data, err := c.tr.Read()
		if err != nil {
			return
		}

		// parse message
		msg, ok := Parse(data)
		if !ok {
			continue
		}

		// match targets for message
		targets := c.match(msg)

		// queue message
		for _, queue := range targets {
			select {
			case queue <- msg:
			case <-time.After(dispatchTimeout):
			}
		}
	}
}

func (c *Channel) write(from Queue, msg Message) error {
	// forward broadcast messages
	if from == nil {
		return c.tr.Write(msg.Build())
	}

	// check owned messages
	if msg.Session != 0 {
		c.mu.Lock()
		owner := c.sessions[msg.Session]
		c.mu.Unlock()
		if owner != nil && owner != from {
			return SessionWrongOwner
		}
	}

	// register write (session opens and closes)
	c.mu.Lock()
	if msg.Session == 0 && msg.Endpoint == 0x0 {
		c.opening[string(msg.Data)] = from
	}
	if msg.Session != 0 && msg.Endpoint == 0xFF {
		c.closing[msg.Session] = from
	}
	c.mu.Unlock()

	// write message
	err := c.tr.Write(msg.Build())
	if err != nil {
		// revert registrations (session opens and closes)
		c.mu.Lock()
		if msg.Session == 0 && msg.Endpoint == 0x0 && c.opening[string(msg.Data)] == from {
			delete(c.opening, string(msg.Data))
		}
		if msg.Session != 0 && msg.Endpoint == 0xFF && c.closing[msg.Session] == from {
			delete(c.closing, msg.Session)
		}
		c.mu.Unlock()

		return err
	}

	return nil
}

func (c *Channel) match(msg Message) []Queue {
	// acquire mutex
	c.mu.Lock()
	defer c.mu.Unlock()

	// handle session open replies
	if msg.Endpoint == 0x0 {
		owner := c.opening[string(msg.Data)]
		if owner != nil {
			delete(c.opening, string(msg.Data))
			_, ok := c.queues[owner]
			if ok {
				c.sessions[msg.Session] = owner
				return []Queue{owner}
			}
		}
	}

	// handle session traffic and closes
	if msg.Session != 0 {
		owner := c.sessions[msg.Session]
		if owner != nil {
			_, ok := c.queues[owner]
			if ok {
				if msg.Endpoint == 0xFF && len(msg.Data) == 0 {
					delete(c.sessions, msg.Session)
					delete(c.closing, msg.Session)
				}
				return []Queue{owner}
			}
			delete(c.sessions, msg.Session)
			delete(c.closing, msg.Session)
		}
		return nil
	}

	// handle broadcast messages
	targets := make([]Queue, 0, len(c.queues))
	for queue := range c.queues {
		targets = append(targets, queue)
	}

	return targets
}
