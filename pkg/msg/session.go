package msg

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"
)

// The SystemEndpoint number.
const SystemEndpoint = 0xFD

// Status represents the status of a session.
type Status uint8

// The available session status flags.
const (
	StatusLocked = Status(1 << 0)
)

// The available session errors.
var (
	SessionInvalidMessage = errors.New("invalid message")
	SessionUnknownMessage = errors.New("unknown message")
	SessionEndpointError  = errors.New("endpoint error")
	SessionLockedError    = errors.New("session locked")
	SessionExpectedAck    = errors.New("expected ack")
)

// Session represents a communication session with a NAOS device.
type Session struct {
	id  uint16
	ch  Channel
	qu  Queue
	mtu uint16
	mu  sync.Mutex
}

// Ack is an error returned when an acknowledgement is received.
var Ack = errors.New("acknowledgement")

// OpenSession opens a new session using the specified channel.
func OpenSession(channel Channel) (*Session, error) {
	// prepare queue
	queue := make(Queue, 64)

	// subscribe to channel
	channel.Subscribe(queue)

	// handle cleanup
	var ok bool
	defer func() {
		if !ok {
			channel.Unsubscribe(queue)
		}
	}()

	// prepare handle
	handle := random(16)

	// begin session
	err := Write(channel, Message{Session: 0, Endpoint: 0x0, Data: handle})
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
		if msg.Endpoint == 0x0 && bytes.Equal(msg.Data, handle) {
			sid = msg.Session
			break
		}
	}

	// set flag
	ok = true

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

// Channel returns the channel used by the session.
func (s *Session) Channel() Channel {
	return s.ch
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
	msg, err := s.read(timeout)
	if err != nil {
		return err
	}

	// verify reply
	if msg.Endpoint != 0xFE || msg.Size() != 1 {
		return fmt.Errorf("invalid message: ping reply")
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
	msg, err := s.read(timeout)
	if err != nil {
		return false, err
	}

	// verify message
	if msg.Endpoint != 0xFE || msg.Size() != 1 {
		return false, fmt.Errorf("invalid message: query reply")
	}

	return msg.Data[0] == 1, nil
}

// Receive waits for a message on the specified endpoint.
func (s *Session) Receive(endpoint uint8, expectAck bool, timeout time.Duration) ([]byte, error) {
	// acquire mutex
	s.mu.Lock()
	defer s.mu.Unlock()

	// read message
	msg, err := s.read(timeout)
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
	msg, err := s.read(ackTimeout)
	if err != nil {
		return err
	}

	// check reply
	if msg.Size() != 1 || msg.Endpoint != 0xFE {
		return fmt.Errorf("invalid message: ack reply")
	} else if msg.Data[0] != 1 {
		return parseError(msg.Data[0])
	}

	return nil
}

// Status returns the status of the session.
func (s *Session) Status(timeout time.Duration) (Status, error) {
	// taking the mutex would deadlock

	// write command
	cmd := Pack("o", uint8(0))
	err := s.Send(0xfd, cmd, 0)
	if err != nil {
		return 0, err
	}

	// await reply
	msg, err := s.Receive(SystemEndpoint, false, timeout)
	if err != nil {
		return 0, err
	}

	// verify reply
	if len(msg) != 1 {
		return 0, fmt.Errorf("invalid message: status reply")
	}

	return Status(msg[0]), nil
}

// Unlock unlocks the session with the specified password.
func (s *Session) Unlock(password string, timeout time.Duration) (bool, error) {
	// taking the mutex would deadlock

	// write command
	cmd := Pack("os", uint8(1), password)
	err := s.Send(SystemEndpoint, cmd, timeout)
	if err != nil {
		return false, err
	}

	// await reply
	msg, err := s.Receive(SystemEndpoint, false, timeout)
	if err != nil {
		return false, err
	}

	// verify reply
	if len(msg) != 1 {
		return false, fmt.Errorf("invalid message: unlock reply")
	}

	return msg[0] == 1, nil
}

func (s *Session) GetMTU(timeout time.Duration) (uint16, error) {
	// taking the mutex would deadlock

	// return cached value
	if s.mtu != 0 {
		return s.mtu, nil
	}

	// write command
	cmd := Pack("o", uint8(2))
	err := s.Send(SystemEndpoint, cmd, 0)
	if err != nil {
		return 0, err
	}

	// await reply
	msg, err := s.Receive(SystemEndpoint, false, timeout)
	if err != nil {
		return 0, err
	}

	// verify reply
	if len(msg) != 2 {
		return 0, fmt.Errorf("invalid message: MTU reply")
	}

	// cache value
	s.mtu = Unpack("h", msg)[0].(uint16)

	return s.mtu, nil
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
	msg, err := s.read(timeout)
	if err != nil {
		return err
	}

	// verify reply
	if msg.Endpoint != 0xFF || msg.Size() > 0 {
		return fmt.Errorf("invalid message: end reply")
	}

	// unsubscribe from channel
	s.ch.Unsubscribe(s.qu)

	return nil
}

func (s *Session) read(timeout time.Duration) (Message, error) {
	for {
		msg, err := Read(s.qu, timeout)
		if err != nil {
			return Message{}, err
		} else if msg.Session == s.id {
			return msg, nil
		}
	}
}

func parseError(num uint8) error {
	switch num {
	case 2:
		return SessionInvalidMessage
	case 3:
		return SessionUnknownMessage
	case 4:
		return SessionEndpointError
	case 5:
		return SessionLockedError
	default:
		return SessionExpectedAck
	}
}
