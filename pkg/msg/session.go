package msg

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"
)

// The available session errors.
var (
	SessionInvalidMessage = errors.New("invalid message")
	SessionUnknownMessage = errors.New("unknown message")
	SessionEndpointError  = errors.New("endpoint error")
	SessionExpectedAck    = errors.New("expected ack")
)

// Session represents a communication session with a NAOS device.
type Session struct {
	id uint16
	ch Channel
	qu Queue
	mu sync.Mutex
}

// Ack is an error returned when an acknowledgement is received.
var Ack = errors.New("acknowledgement")

// OpenSession opens a new session using the specified channel.
func OpenSession(channel Channel) (*Session, error) {
	// prepare queue
	queue := make(Queue, 16)

	// subscribe to channel
	channel.Subscribe(queue)

	// prepare handle
	handle := random(16)

	// begin session
	err := Write(channel, Message{Session: 0, Endpoint: 0x00, Data: handle})
	if err != nil {
		return nil, err
	}

	// await reply
	var sid uint16
	for {
		msg, err := Read(queue, 10*time.Second)
		if err != nil {
			return nil, err
		}
		if msg.Endpoint == 0x00 && bytes.Equal(msg.Data, handle) {
			sid = msg.Session
			break
		}
	}

	return &Session{
		id: sid,
		ch: channel,
		qu: queue,
	}, nil
}

// ID returns the session ID.
func (s *Session) ID() uint16 {
	return s.id
}

// Ping verifies and keeps the session alive.
func (s *Session) Ping(timeout time.Duration) error {
	// acquire mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// write command
	err := Write(s.ch, Message{Session: s.id, Endpoint: 0xFE, Data: nil})
	if err != nil {
		return err
	}

	// await reply
	msg, err := Read(s.qu, timeout)
	if err != nil {
		return err
	}

	// verify reply
	if msg.Endpoint != 0xFE || msg.Size() != 1 {
		return fmt.Errorf("invalid message")
	} else if msg.Data[0] != 1 {
		return fmt.Errorf("session error: %d", msg.Data[0])
	}

	return nil
}

// Query tests existence of an endpoint.
func (s *Session) Query(endpoint uint8, timeout time.Duration) (bool, error) {
	// acquire mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// write command
	err := Write(s.ch, Message{Session: s.id, Endpoint: endpoint, Data: nil})
	if err != nil {
		return false, err
	}

	// read reply
	msg, err := Read(s.qu, timeout)
	if err != nil {
		return false, err
	}

	// verify message
	if msg.Endpoint != 0xFE || msg.Size() != 1 {
		return false, fmt.Errorf("invalid message")
	}

	return msg.Data[0] == 1, nil
}

// Receive waits for a message on the specified endpoint.
func (s *Session) Receive(endpoint uint8, expectAck bool, timeout time.Duration) ([]byte, error) {
	// acquire mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// read message
	msg, err := Read(s.qu, timeout)
	if err != nil {
		return nil, err
	}

	// TODO: Check session ID.

	// handle acknowledgements
	if msg.Endpoint == 0xFE {
		// check size
		if msg.Size() != 1 {
			return nil, fmt.Errorf("invalid ack size: %d", msg.Size())
		}

		// check if OK
		if msg.Data[0] == 1 {
			if expectAck {
				return nil, Ack
			} else {
				return nil, fmt.Errorf("unexpected ack")
			}
		}

		return nil, parseError(msg.Data[0])
	}

	// check endpoint
	if msg.Endpoint != endpoint {
		return nil, fmt.Errorf("unexpected endpoint: %d", msg.Endpoint)
	}

	return msg.Data, nil
}

// Send sends a message to the specified endpoint.
func (s *Session) Send(endpoint uint8, data []byte, ackTimeout time.Duration) error {
	// acquire mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// write message
	err := Write(s.ch, Message{Session: s.id, Endpoint: endpoint, Data: data})
	if err != nil {
		return err
	}

	// return if timeout is zero
	if ackTimeout == 0 {
		return nil
	}

	// await reply
	msg, err := Read(s.qu, ackTimeout)
	if err != nil {
		return err
	}

	// check reply
	if msg.Size() != 1 || msg.Endpoint != 0xFE {
		return fmt.Errorf("invalid message")
	} else if msg.Data[0] != 1 {
		return parseError(msg.Data[0])
	}

	return nil
}

// End closes the session.
func (s *Session) End(timeout time.Duration) error {
	// acquire mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// write command
	err := Write(s.ch, Message{Session: s.id, Endpoint: 0xFF, Data: nil})
	if err != nil {
		return err
	}

	// read reply
	msg, err := Read(s.qu, timeout)
	if err != nil {
		return err
	}

	// verify reply
	if msg.Endpoint != 0xFF || msg.Size() > 0 {
		return fmt.Errorf("invalid message")
	}

	// unsubscribe from channel
	s.ch.Unsubscribe(s.qu)

	return nil
}

func parseError(num uint8) error {
	switch num {
	case 2:
		return SessionInvalidMessage
	case 3:
		return SessionUnknownMessage
	case 4:
		return SessionEndpointError
	default:
		return SessionExpectedAck
	}
}
