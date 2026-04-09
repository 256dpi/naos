package msg

import "time"

const updateEndpoint = 0x2

// Update performs a firmware update.
func Update(s *Session, image []byte, report func(int), timeout time.Duration) error {
	// send "begin" command
	cmd := Pack("oi", uint8(0), uint32(len(image)))
	err := s.Send(updateEndpoint, cmd, timeout)
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
	for offset < len(image) {
		// determine chunk size and chunk data
		chunkSize := min(int(mtu), len(image)-offset)
		chunkData := image[offset : offset+chunkSize]

		// determine acked
		acked := num%width == 0

		// send "write" command
		cmd = Pack("ooib", uint8(1), b2u(acked), uint32(offset), chunkData)
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
	cmd = Pack("o", uint8(3))
	err = s.Send(updateEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}
