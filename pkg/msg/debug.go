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

// The available echo flags.
const (
	echoRepeat  = 1 << 0
	echoDiscard = 1 << 1
	echoSilent  = 1 << 2
)

// echoCommand prepares an echo command with the given flags, count and
// payload size. The payload is filled with a rolling byte pattern.
func echoCommand(flags uint8, count uint16, size int) []byte {
	cmd := make([]byte, 4+size)
	cmd[0] = 5
	cmd[1] = flags
	binary.LittleEndian.PutUint16(cmd[2:], count)
	for i := 4; i < len(cmd); i++ {
		cmd[i] = byte(i)
	}
	return cmd
}

// Echo sends the provided data to the device and returns the echoed reply.
func Echo(s *Session, data []byte, timeout time.Duration) ([]byte, error) {
	// send command
	cmd := echoCommand(0, 1, len(data))
	copy(cmd[4:], data)
	err := s.Send(debugEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// receive reply
	reply, err := s.Receive(debugEndpoint, false, timeout)
	if err != nil {
		return nil, err
	}

	// receive ack
	_, err = s.Receive(debugEndpoint, true, timeout)
	if err == nil {
		return nil, fmt.Errorf("missing ack")
	} else if !errors.Is(err, Ack) {
		return nil, err
	}

	return reply, nil
}

// MeasureDownload measures downstream throughput by requesting a stream of
// repeated echo replies of the given payload size. It returns the number of
// payload bytes received and the elapsed time.
func MeasureDownload(s *Session, size, count int, timeout time.Duration) (int, time.Duration, error) {
	// check arguments
	if size <= 0 || count <= 0 || count > 0xFFFF {
		return 0, 0, fmt.Errorf("invalid size or count")
	}

	// send command
	cmd := echoCommand(echoRepeat, uint16(count), size)
	start := time.Now()
	err := s.Send(debugEndpoint, cmd, 0)
	if err != nil {
		return 0, 0, err
	}

	// receive replies until ack
	var total int
	for {
		reply, err := s.Receive(debugEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			break
		} else if err != nil {
			return 0, 0, err
		}
		if len(reply) != size {
			return 0, 0, fmt.Errorf("invalid echo reply")
		}
		total += size
	}

	// verify total
	if total != size*count {
		return 0, 0, fmt.Errorf("incomplete echo stream")
	}

	return total, time.Since(start), nil
}

// MeasureUpload measures upstream throughput by sending batches of discarded
// echo commands of the given payload size for at least the specified duration.
// All commands but the last of each batch are silent, while the acknowledged
// last command acts as a barrier that confirms in-order processing of the
// whole batch. It returns the number of payload bytes sent and the elapsed
// time.
func MeasureUpload(s *Session, size, window int, duration, timeout time.Duration) (int, time.Duration, error) {
	// check arguments
	if size <= 0 || window <= 0 {
		return 0, 0, fmt.Errorf("invalid size or window")
	}

	// prepare commands
	silentCmd := echoCommand(echoDiscard|echoSilent, 0, size)
	ackedCmd := echoCommand(echoDiscard, 0, size)

	// run batched echo exchanges until the deadline has passed, but always
	// send at least one batch
	start := time.Now()
	deadline := start.Add(duration)
	var total int
	for total == 0 || time.Now().Before(deadline) {
		// send silent commands followed by a final acknowledged command
		for i := 0; i < window; i++ {
			cmd := silentCmd
			if i == window-1 {
				cmd = ackedCmd
			}
			err := s.Send(debugEndpoint, cmd, 0)
			if err != nil {
				return 0, 0, err
			}
		}

		// receive ack
		_, err := s.Receive(debugEndpoint, true, timeout)
		if err == nil {
			return 0, 0, fmt.Errorf("unexpected reply")
		} else if !errors.Is(err, Ack) {
			return 0, 0, err
		}
		total += window * size
	}

	return total, time.Since(start), nil
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
