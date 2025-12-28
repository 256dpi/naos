package msg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const debugEndpoint = 0x7

// CheckCoredump returns the size and reason of the coredump.
func CheckCoredump(s *Session, timeout time.Duration) (uint32, string, error) {
	// send command
	cmd := []byte{0}
	err := s.Send(debugEndpoint, cmd, 0)
	if err != nil {
		return 0, "", err
	}

	// receive reply
	reply, err := s.Receive(debugEndpoint, false, timeout)
	if err != nil {
		return 0, "", err
	}

	// verify reply
	if len(reply) < 4 {
		return 0, "", fmt.Errorf("invalid reply")
	}

	// parse size
	size := binary.LittleEndian.Uint32(reply)

	// parse reason
	reason := string(reply[4:])

	return size, reason, nil
}

// ReadCoredump reads the coredump.
func ReadCoredump(s *Session, offset, length uint32, timeout time.Duration) ([]byte, error) {
	// send command
	cmd := make([]byte, 9)
	cmd[0] = 1
	binary.LittleEndian.PutUint32(cmd[1:], offset)
	binary.LittleEndian.PutUint32(cmd[5:], length)
	err := s.Send(debugEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// prepare data
	var data []byte

	for {
		// receive reply or return data on ack
		reply, err := s.Receive(debugEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			break
		} else if err != nil {
			return nil, err
		}

		// verify reply
		if len(reply) < 4 {
			return nil, fmt.Errorf("invalid reply")
		}

		// get chunk offset
		chunkOffset := binary.LittleEndian.Uint32(reply[:4])

		// verify chunk offset
		if chunkOffset != offset+uint32(len(data)) {
			return nil, fmt.Errorf("invalid chunk offset")
		}

		// append chunk data
		data = append(data, reply[4:]...)
	}

	return data, nil
}

// DeleteCoredump deletes the coredump.
func DeleteCoredump(s *Session, timeout time.Duration) error {
	// send command
	cmd := []byte{2}
	err := s.Send(debugEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}

// StartLog subscribes to log messages.
func StartLog(s *Session, timeout time.Duration) error {
	// send command
	cmd := []byte{3}
	err := s.Send(debugEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}

// StopLog unsubscribes from log messages.
func StopLog(s *Session, timeout time.Duration) error {
	// send command
	cmd := []byte{4}
	err := s.Send(debugEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}

// ReceiveLog receives a log message.
func ReceiveLog(s *Session, timeout time.Duration) (string, error) {
	// receive message
	data, err := s.Receive(debugEndpoint, false, timeout)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
