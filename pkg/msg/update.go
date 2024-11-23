package msg

import (
	"fmt"
	"time"
)

// The UpdateEndpoint number.
const UpdateEndpoint = 0x02

// Update performs a firmware update.
func Update(s *Session, image []byte, report func(int), timeout time.Duration) error {
	// prepare "begin" command
	cmd := pack("oi", 0, len(image))

	// write "begin" command
	err := s.Send(UpdateEndpoint, cmd, 0)
	if err != nil {
		return err
	}

	// receive value
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

		// prepare "write" command
		cmd = pack("ob", 1, b2u(acked), chunkData)

		// send "write" command
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

	// prepare "finish" command
	cmd = pack("o", 3)

	// write "finish" command
	err = s.Send(UpdateEndpoint, cmd, 0)
	if err != nil {
		return err
	}

	// receive value
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
