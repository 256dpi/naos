package msg

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetTime(t *testing.T) {
	reply := make([]byte, 8)
	binary.LittleEndian.PutUint64(reply, uint64(1700000000000))

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: timeEndpoint, Data: []byte{0}}),
		send(Message{Endpoint: timeEndpoint, Data: reply}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	got, err := GetTime(s, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, time.UnixMilli(1700000000000), got)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestSetTime(t *testing.T) {
	cmd := make([]byte, 9)
	cmd[0] = 1
	binary.LittleEndian.PutUint64(cmd[1:], uint64(1700000000000))

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: timeEndpoint, Data: cmd}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = SetTime(s, time.UnixMilli(1700000000000), time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestGetTimeInfo(t *testing.T) {
	reply := make([]byte, 4)
	binary.LittleEndian.PutUint32(reply, uint32(int32(3600)))

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: timeEndpoint, Data: []byte{2}}),
		send(Message{Endpoint: timeEndpoint, Data: reply}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	offset, err := GetTimeInfo(s, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, time.Hour, offset)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
