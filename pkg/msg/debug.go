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

// StreamLog streams log messages and calls the provided function for each
// message until the stop channel is closed.
func StreamLog(s *Session, stop chan struct{}, fn func(string)) error {
	// start log, with checking ack
	err := s.Send(debugEndpoint, []byte{3}, 5*time.Second)
	if err != nil {
		return err
	}

	// mark last message
	last := time.Now()

	for {
		// stop log if requested
		select {
		case <-stop:
			return s.Send(debugEndpoint, []byte{4}, time.Second)
		default:
		}

		// receive log message
		data, err := s.Receive(debugEndpoint, true, time.Second)

		// yield and continue on success
		if err == nil {
			last = time.Now()
			fn(string(data))
			continue
		}

		// ignore ack
		if errors.Is(err, Ack) {
			last = time.Now()
			continue
		}

		// stop on any error except timeout
		if !errors.Is(err, ErrTimeout) {
			return err
		}

		/* error is timeout */

		// continue if a message was received recently
		if time.Since(last) < 20*time.Second {
			continue
		}

		// otherwise, restart log without checking ack
		err = s.Send(debugEndpoint, []byte{3}, 0)
		if err != nil {
			return err
		}

		// update last message time
		last = time.Now()
	}
}
