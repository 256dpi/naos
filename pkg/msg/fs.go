package msg

import (
	"errors"
	"fmt"
	"time"
)

const fsEndpoint = 0x3

// FSInfo describes a file system entry.
type FSInfo struct {
	Name  string
	IsDir bool
	Size  uint32
}

// StatPath retrieves information about a file system entry.
func StatPath(s *Session, path string, timeout time.Duration) (*FSInfo, error) {
	// send command
	cmd := pack("os", uint8(0), path)
	err := fsSend(s, cmd, false, timeout)
	if err != nil {
		return nil, err
	}

	// await reply
	reply, err := fsReceive(s, false, timeout)
	if err != nil {
		return nil, err
	}

	// verify "info" reply
	if len(reply) == 6 && reply[0] != 1 {
		return nil, fmt.Errorf("invalid message")
	}

	// unpack "info" reply
	args := unpack("oi", reply[1:])

	return &FSInfo{
		IsDir: args[0].(uint8) == 1,
		Size:  args[1].(uint32),
	}, nil
}

// ListDir retrieves a list of file system entries in a directory.
func ListDir(s *Session, dir string, timeout time.Duration) ([]FSInfo, error) {
	// send command
	cmd := pack("os", uint8(1), dir)
	err := fsSend(s, cmd, false, timeout)
	if err != nil {
		return nil, nil
	}

	// prepare infos
	var infos []FSInfo

	for {
		// await reply
		reply, err := fsReceive(s, true, timeout)
		if errors.Is(err, Ack) {
			return infos, nil
		} else if err != nil {
			return nil, err
		}

		// verify "info" reply
		if len(reply) < 7 && reply[0] != 1 {
			return nil, fmt.Errorf("invalid message")
		}

		// unpack "info" reply
		args := unpack("ois", reply[1:])

		// add info
		infos = append(infos, FSInfo{
			Name:  args[2].(string),
			IsDir: args[0].(uint8) == 1,
			Size:  args[1].(uint32),
		})
	}
}

// ReadFile reads the contents of a file.
func ReadFile(s *Session, file string, report func(uint32), timeout time.Duration) ([]byte, error) {
	// stat file
	info, err := StatPath(s, file, timeout)
	if err != nil {
		return nil, err
	}

	// prepare buffer
	data := make([]byte, info.Size)

	// read file in chunks of 5 KB
	var offset uint32
	for offset < info.Size {
		// determine length
		length := min(5000, info.Size-offset)

		// read range
		rng, err := ReadFileRange(s, file, offset, length, func(pos uint32) {
			if report != nil {
				report(offset + pos)
			}
		}, timeout)
		if err != nil {
			return nil, err
		}

		// append range
		copy(data[offset:], rng)
		offset += uint32(len(rng))
	}

	return data, nil
}

// ReadFileRange reads a range of bytes from a file.
func ReadFileRange(s *Session, file string, offset, length uint32, report func(uint32), timeout time.Duration) ([]byte, error) {
	// send "open" command
	cmd := pack("oos", uint8(2), uint8(0), file)
	err := fsSend(s, cmd, true, timeout)
	if err != nil {
		return nil, err
	}

	// send "read" command
	cmd = pack("oii", uint8(3), offset, length)
	err = fsSend(s, cmd, false, timeout)
	if err != nil {
		return nil, err
	}

	// prepare data
	data := make([]byte, length)

	// prepare counter
	var count uint32

	for {
		// await reply
		reply, err := fsReceive(s, true, timeout)
		if errors.Is(err, Ack) {
			break
		} else if err != nil {
			return nil, err
		}

		// verify "chunk" reply
		if len(reply) < 5 && reply[0] != 2 {
			return nil, fmt.Errorf("invalid message")
		}

		// unpack offset
		replyOffset := unpack("i", reply[1:])[0].(uint32)

		// verify offset
		if replyOffset != offset+count {
			return nil, fmt.Errorf("invalid offset")
		}

		// append data
		copy(data[count:], reply[5:])

		// increment
		count += uint32(len(reply) - 5)

		// report length
		if report != nil {
			report(count)
		}
	}

	// send "close" command
	cmd = pack("o", uint8(5))
	err = fsSend(s, cmd, true, timeout)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// WriteFile writes data to a file.
func WriteFile(s *Session, file string, data []byte, report func(uint32), timeout time.Duration) error {
	// send "create" command
	cmd := pack("oos", uint8(2), uint8((1<<0)|(1<<2)), file)
	err := fsSend(s, cmd, true, timeout)
	if err != nil {
		return err
	}

	// get width
	width := s.Channel().Width()

	// get MTU
	mtu, err := s.GetMTU(time.Second)
	if err != nil {
		return err
	}

	// subtract overhead
	mtu -= 6

	// write data in chunks
	num := 0
	offset := 0
	for offset < len(data) {
		// determine chunk size and chunk data
		chunkSize := min(int(mtu), len(data)-offset)
		chunkData := data[offset : offset+chunkSize]

		// determine mode
		acked := num%width == 0

		// prepare "write" command (acked or silent & sequential)
		cmd := pack("ooib", uint8(4), b2v(acked, uint8(0), uint8(1<<0|1<<1)), uint32(offset), chunkData)

		// send "write" command
		err = fsSend(s, cmd, false, 0)
		if err != nil {
			return err
		}

		// receive ack or "error" replies
		if acked {
			_, err := fsReceive(s, true, timeout)
			if err != nil && !errors.Is(err, Ack) {
				return err
			}
		}

		// increment offset
		offset += chunkSize

		// report offset
		if report != nil {
			report(uint32(offset))
		}

		// increment count
		num++
	}

	// send "close" command
	cmd = pack("o", uint8(5))
	err = fsSend(s, cmd, true, timeout)
	if err != nil {
		return err
	}

	return nil
}

// RenamePath renames a file system entry.
func RenamePath(s *Session, from, to string, timeout time.Duration) error {
	// send command
	cmd := pack("osos", uint8(6), from, uint8(0), to)
	err := fsSend(s, cmd, true, timeout)
	if err != nil {
		return err
	}

	return nil
}

// RemovePath removes a file system entry.
func RemovePath(s *Session, path string, timeout time.Duration) error {
	// send command
	cmd := pack("os", uint8(7), path)
	err := fsSend(s, cmd, true, timeout)
	if err != nil {
		return err
	}

	return nil
}

// SHA256File retrieves the SHA-256 hash of a file.
func SHA256File(s *Session, file string, timeout time.Duration) ([]byte, error) {
	// send command
	cmd := pack("os", uint8(8), file)
	err := fsSend(s, cmd, false, timeout)
	if err != nil {
		return nil, err
	}

	// await reply
	reply, err := fsReceive(s, false, timeout)
	if err != nil {
		return nil, err
	}

	// verify "hash" reply
	if len(reply) != 33 && reply[0] != 3 {
		return nil, fmt.Errorf("invalid message")
	}

	// return hash
	return reply[1:], nil
}

// MakePath creates a directory path.
func MakePath(s *Session, path string, timeout time.Duration) error {
	// send command
	cmd := pack("os", uint8(9), path)
	err := fsSend(s, cmd, true, timeout)
	if err != nil {
		return err
	}

	return nil
}

/* Helpers */

func fsReceive(s *Session, expectAck bool, timeout time.Duration) ([]byte, error) {
	// await reply
	reply, err := s.Receive(fsEndpoint, expectAck, timeout)
	if err != nil {
		return nil, err
	}

	// handle errors
	if reply[0] == 0 {
		return nil, fmt.Errorf("posix error: %s", reply[1:])
	}

	return reply, nil
}

func fsSend(s *Session, data []byte, awaitAck bool, timeout time.Duration) error {
	// send data
	if awaitAck {
		return s.Send(fsEndpoint, data, timeout)
	} else {
		return s.Send(fsEndpoint, data, 0)
	}
}
