package msg

import (
	"encoding/binary"
	"fmt"
	"time"
)

const timeEndpoint = 0x9

// GetTime returns the device's current wall-clock time in UTC at millisecond
// resolution.
func GetTime(s *Session, timeout time.Duration) (time.Time, error) {
	// send command
	err := s.Send(timeEndpoint, []byte{0}, 0)
	if err != nil {
		return time.Time{}, err
	}

	// receive reply
	reply, err := s.Receive(timeEndpoint, false, timeout)
	if err != nil {
		return time.Time{}, err
	}

	// verify reply
	if len(reply) != 8 {
		return time.Time{}, fmt.Errorf("invalid reply")
	}

	// parse epoch milliseconds
	ms := int64(binary.LittleEndian.Uint64(reply))

	return time.UnixMilli(ms), nil
}

// SetTime sets the device's wall-clock time in UTC at millisecond resolution.
func SetTime(s *Session, t time.Time, timeout time.Duration) error {
	// build command
	cmd := make([]byte, 9)
	cmd[0] = 1
	binary.LittleEndian.PutUint64(cmd[1:], uint64(t.UnixMilli()))

	// send command
	err := s.Send(timeEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}

// GetTimeInfo returns the device's current timezone offset from UTC.
func GetTimeInfo(s *Session, timeout time.Duration) (time.Duration, error) {
	// send command
	err := s.Send(timeEndpoint, []byte{2}, 0)
	if err != nil {
		return 0, err
	}

	// receive reply
	reply, err := s.Receive(timeEndpoint, false, timeout)
	if err != nil {
		return 0, err
	}

	// verify reply
	if len(reply) != 4 {
		return 0, fmt.Errorf("invalid reply")
	}

	// parse offset in seconds
	offset := int32(binary.LittleEndian.Uint32(reply))

	return time.Duration(offset) * time.Second, nil
}
