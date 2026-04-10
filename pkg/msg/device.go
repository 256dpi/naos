package msg

import (
	"errors"
	"time"
)

// ErrTimeout is returned when a timeout occurs.
var ErrTimeout = errors.New("timeout")

// Device represents a device that can be communicated with.
type Device interface {
	// ID returns a stable identifier for the device.
	ID() string

	// Open opens a channel to the device. An opened channel must fail or be
	// closed before another channel can be opened.
	Open() (*Channel, error)
}

// Queue is used to receive messages from a channel.
type Queue chan Message

// Read reads a message from the queue.
func (q Queue) Read(timeout time.Duration) (Message, error) {
	select {
	case msg := <-q:
		return msg, nil
	case <-time.After(timeout):
		return Message{}, ErrTimeout
	}
}

// Message represents a message exchanged between a device and a client.
type Message struct {
	Session  uint16
	Endpoint uint8
	Data     []byte
}

// Parse decodes raw message bytes.
func Parse(data []byte) (Message, bool) {
	// check header
	if len(data) < 4 || data[0] != 1 {
		return Message{}, false
	}

	// unpack header
	args, err := Unpack("hob", data[1:])
	if err != nil {
		return Message{}, false
	}

	return Message{
		Session:  args[0].(uint16),
		Endpoint: args[1].(uint8),
		Data:     args[2].([]byte),
	}, true
}

// Build encodes the message to its wire format.
func (m *Message) Build() []byte {
	return Pack("ohob", uint8(1), m.Session, m.Endpoint, m.Data)
}

// Size returns the size of the message.
func (m *Message) Size() int {
	return len(m.Data)
}
