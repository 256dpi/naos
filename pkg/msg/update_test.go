package msg

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdate(t *testing.T) {
	imageData := bytes.Repeat([]byte{0xBB}, 50)

	dev := newTestDevice(t, 42, []testMessage{
		// begin
		receive(Message{Endpoint: updateEndpoint, Data: Pack("oi", uint8(0), uint32(50))}),
		ack(),
		// GetMTU
		receive(Message{Endpoint: SystemEndpoint, Data: Pack("o", uint8(2))}),
		send(Message{Endpoint: SystemEndpoint, Data: Pack("h", uint16(30))}),
		// chunk 0: 24 bytes, acked (b2u(true)=1)
		receive(Message{Endpoint: updateEndpoint, Data: Pack("ooib", uint8(1), uint8(1), uint32(0), imageData[0:24])}),
		ack(),
		// chunk 1: 24 bytes, not acked (b2u(false)=0)
		receive(Message{Endpoint: updateEndpoint, Data: Pack("ooib", uint8(1), uint8(0), uint32(24), imageData[24:48])}),
		// chunk 2: 2 bytes, not acked (b2u(false)=0)
		receive(Message{Endpoint: updateEndpoint, Data: Pack("ooib", uint8(1), uint8(0), uint32(48), imageData[48:50])}),
		// finish
		receive(Message{Endpoint: updateEndpoint, Data: Pack("o", uint8(3))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	var progress []int
	err = Update(s, imageData, func(pos int) {
		progress = append(progress, pos)
	}, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []int{24, 48, 50}, progress)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
