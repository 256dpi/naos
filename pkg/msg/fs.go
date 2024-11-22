package msg

import (
	"errors"
	"fmt"
	"time"
)

// The FSEndpoint number.
const FSEndpoint = 0x03

// FSInfo describes a file system entry.
type FSInfo struct {
	Name  string
	IsDir bool
	Size  uint32
}

// StatPath retrieves information about a file system entry.
func StatPath(s *Session, path string, timeout time.Duration) (*FSInfo, error) {
	// send command
	err := fsSend(s, pack("os", uint8(0), path), false, timeout)
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
	var isDir uint8
	var size uint32
	unpack("oi", reply[1:], &isDir, &size)

	return &FSInfo{
		Name:  "",
		IsDir: isDir == 1,
		Size:  size,
	}, nil
}

// ListDir retrieves a list of file system entries in a directory.
func ListDir(s *Session, path string, timeout time.Duration) ([]FSInfo, error) {
	// send command
	err := fsSend(s, pack("os", uint8(1), path), false, timeout)
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
		var isDir uint8
		var size uint32
		var name string
		unpack("ois", reply[1:], &isDir, &size, &name)

		// add info
		infos = append(infos, FSInfo{
			Name:  name,
			IsDir: isDir == 1,
			Size:  size,
		})
	}
}

// ReadFile reads the contents of a file.
func ReadFile(s *Session, path string, report func(uint32), timeout time.Duration) ([]byte, error) {
	// stat file
	info, err := StatPath(s, path, timeout)
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
		rng, err := ReadFileRange(s, path, offset, length, func(pos uint32) {
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
func ReadFileRange(s *Session, path string, offset, length uint32, report func(uint32), timeout time.Duration) ([]byte, error) {
	// send "open" command
	err := fsSend(s, pack("oos", uint8(2), uint8(0), path), true, timeout)
	if err != nil {
		return nil, err
	}

	// send "read" command
	err = fsSend(s, pack("oii", uint8(3), offset, length), false, timeout)
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
		var replyOffset uint32
		unpack("i", reply[1:], &replyOffset)

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
	err = fsSend(s, pack("o", uint8(5)), true, timeout)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// WriteFile writes data to a file.
func WriteFile(s *Session, path string, data []byte, report func(uint32), timeout time.Duration) error {
	// send "create" command
	err := fsSend(s, pack("oos", uint8(2), uint8((1<<0)|(1<<2)), path), true, timeout)
	if err != nil {
		return err
	}

	// TODO: Determine channel MTU.

	// write data in 500-byte chunks
	num := 0
	offset := 0
	for offset < len(data) {
		// determine chunk size and chunk data
		chunkSize := min(500, len(data)-offset)
		chunkData := data[offset : offset+chunkSize]

		// determine mode
		acked := num%10 == 0
		writeMode := 0
		if !acked {
			writeMode = 1<<0 | 1<<1
		}

		// prepare "write" command (acked or silent & sequential)
		cmd := pack("ooib", uint8(4), uint8(writeMode), uint32(offset), chunkData)

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
	err = fsSend(s, pack("o", uint8(5)), true, timeout)
	if err != nil {
		return err
	}

	return nil
}

// RenamePath renames a file system entry.
func RenamePath(s *Session, from, to string, timeout time.Duration) error {
	// send command
	err := fsSend(s, pack("osos", uint8(6), from, uint8(0), to), false, timeout)
	if err != nil {
		return err
	}

	// await reply
	_, err = fsReceive(s, true, timeout)
	if err != nil && !errors.Is(err, Ack) {
		return err
	}

	return nil
}

// RemovePath removes a file system entry.
func RemovePath(s *Session, path string, timeout time.Duration) error {
	// send command
	err := fsSend(s, pack("os", uint8(7), path), true, timeout)
	if err != nil {
		return err
	}

	return nil
}

// SHA256File retrieves the SHA-256 hash of a file.
func SHA256File(s *Session, path string, timeout time.Duration) ([]byte, error) {
	// send command
	err := fsSend(s, pack("os", uint8(8), path), false, timeout)
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

func fsReceive(s *Session, expectAck bool, timeout time.Duration) ([]byte, error) {
	// await reply
	reply, err := s.Receive(FSEndpoint, expectAck, timeout)
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
		return s.Send(FSEndpoint, data, timeout)
	} else {
		return s.Send(FSEndpoint, data, 0)
	}
}
