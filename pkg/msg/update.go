package msg

import (
	"fmt"
	"time"
)

const updateEndpoint = 0x2

// Update performs a firmware update.
func Update(s *Session, image []byte, report func(int), timeout time.Duration) error {
	// send "begin" command
	cmd := pack("oi", uint8(0), uint32(len(image)))
	err := s.Send(updateEndpoint, cmd, 0)
	if err != nil {
		return err
	}

	// receive reply
	reply, err := s.Receive(updateEndpoint, false, timeout)
	if err != nil {
		return err
	}

	// verify reply
	if len(reply) != 1 || reply[0] != 0 {
		return fmt.Errorf("invalid message")
	}

	// get MTU
	mtu, err := s.GetMTU(time.Second)
	if err != nil {
		return err
	}

	// write data in chunks
	num := 0
	offset := 0
	for offset < len(image) {
		// determine chunk size and chunk data
		chunkSize := min(int(mtu), len(image)-offset)
		chunkData := image[offset : offset+chunkSize]

		// determine acked
		acked := num%10 == 0

		// send "write" command
		cmd = pack("oob", uint8(1), b2u(acked), chunkData)
		err = s.Send(updateEndpoint, cmd, b2v(acked, timeout, 0))
		if err != nil {
			return err
		}

		// increment offset
		offset += chunkSize

		// report offset
		if report != nil {
			report(offset)
		}

		// increment count
		num += 1
	}

	// send "finish" command
	cmd = pack("o", uint8(3))
	err = s.Send(updateEndpoint, cmd, 0)
	if err != nil {
		return err
	}

	// receive reply
	reply, err = s.Receive(updateEndpoint, false, timeout)
	if err != nil {
		return err
	}

	// verify reply
	if len(reply) != 1 || reply[0] != 1 {
		return fmt.Errorf("invalid message")
	}

	return nil
}
