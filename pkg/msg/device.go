package msg

import (
	"encoding/binary"
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
		return Message{}, errors.New("invalid message")
	}

	return Message{
		Session:  binary.LittleEndian.Uint16(data[1:3]),
		Endpoint: data[3],
		Data:     data[4:],
	}, nil
}

// Write writes a message to the channel.
func Write(ch Channel, msg Message) error {
	// prepare data
	data := pack("ohob", uint8(1), msg.Session, msg.Endpoint, msg.Data)

	// write data
	err := ch.Write(data)
	if err != nil {
		return err
	}

	return nil
}
