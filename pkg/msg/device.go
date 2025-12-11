package msg

import (
	"errors"
	"time"
)

// Device represents a device that can be communicated with.
type Device interface {
	// ID returns a stable identifier for the device.
	ID() string

	// Open opens a channel to the device. An opened channel must fail or be
	// closed before another channel can be opened.
	Open() (Channel, error)
}

// Queue is used to receive messages from a channel.
type Queue chan []byte

// Channel provides the mechanism to exchange messages between a device and a client.
type Channel interface {
	Width() int
	Device() Device
	Subscribe(Queue)
	Unsubscribe(Queue)
	Write([]byte) error
	Close()
}

// Message represents a message exchanged between a device and a client.
type Message struct {
	Session  uint16
	Endpoint uint8
	Data     []byte
}

// Size returns the size of the message.
func (m *Message) Size() int {
	return len(m.Data)
}

// Read reads a message from the queue.
func Read(q Queue, timeout time.Duration) (Message, error) {
	// read data
	var data []byte
	select {
	case data = <-q:
	case <-time.After(timeout):
		return Message{}, errors.New("timeout")
	}

	// check length and version
	if len(data) < 4 || data[0] != 1 {
		return Message{}, errors.New("invalid message: length or version")
	}

	// unpack message
	args := Unpack("hob", data[1:])

	return Message{
		Session:  args[0].(uint16),
		Endpoint: args[1].(uint8),
		Data:     args[2].([]byte),
	}, nil
}

// Write writes a message to the channel.
func Write(ch Channel, msg Message) error {
	// prepare data
	data := Pack("ohob", uint8(1), msg.Session, msg.Endpoint, msg.Data)

	// write data
	err := ch.Write(data)
	if err != nil {
		return err
	}

	return nil
}
