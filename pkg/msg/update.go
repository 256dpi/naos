package msg

import (
	"fmt"
	"time"
)

// The UpdateEndpoint number.
const UpdateEndpoint = 0x2

// Update performs a firmware update.
func Update(s *Session, image []byte, report func(int), timeout time.Duration) error {
	// send "begin" command
	cmd := pack("oi", uint8(0), uint32(len(image)))
	err := s.Send(UpdateEndpoint, cmd, 0)
	if err != nil {
		return err
	}

	// receive reply
	reply, err := s.Receive(UpdateEndpoint, false, timeout)
	if err != nil {
		return err
	}

	// verify reply
	if len(reply) != 1 || reply[0] != 0 {
		return fmt.Errorf("invalid message")
	}

	// TODO: Dynamically determine channel MTU?

	// write data in 500-byte chunks
	num := 0
	offset := 0
	for offset < len(image) {
		// determine chunk size and chunk data
		chunkSize := min(500, len(image)-offset)
		chunkData := image[offset : offset+chunkSize]

		// determine acked
		acked := num%10 == 0

		// send "write" command
		cmd = pack("oob", uint8(1), b2u(acked), chunkData)
		err = s.Send(UpdateEndpoint, cmd, b2v(acked, timeout, 0))
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
	err = s.Send(UpdateEndpoint, cmd, 0)
	if err != nil {
		return err
	}

	// receive reply
	reply, err = s.Receive(UpdateEndpoint, false, timeout)
	if err != nil {
		return err
	}

	// verify reply
	if len(reply) != 1 || reply[0] != 1 {
		return fmt.Errorf("invalid message")
	}

	return nil
}
