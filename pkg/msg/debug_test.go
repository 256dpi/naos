package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckCoredump(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: debugEndpoint, Data: []byte{0}}),
		send(Message{Endpoint: debugEndpoint, Data: Pack("is", uint32(1024), "panic")}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	size, reason, err := CheckCoredump(s, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, uint32(1024), size)
	assert.Equal(t, "panic", reason)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadCoredump(t *testing.T) {
	coredumpData := []byte("0123456789")

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: debugEndpoint, Data: Pack("oii", uint8(1), uint32(0), uint32(10))}),
		send(Message{Endpoint: debugEndpoint, Data: Pack("ib", uint32(0), coredumpData)}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	data, err := ReadCoredump(s, 0, 10, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, coredumpData, data)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestDeleteCoredump(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: debugEndpoint, Data: []byte{2}}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = DeleteCoredump(s, time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestStreamLog(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		// start log
		receive(Message{Endpoint: debugEndpoint, Data: []byte{3}}),
		ack(),
		// log messages
		send(Message{Endpoint: debugEndpoint, Data: []byte("hello")}),
		send(Message{Endpoint: debugEndpoint, Data: []byte("world")}),
		// stop log
		receive(Message{Endpoint: debugEndpoint, Data: []byte{4}}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	stop := make(chan struct{})
	var logs []string
	done := make(chan error, 1)
	go func() {
		done <- StreamLog(s, stop, func(msg string) {
			logs = append(logs, msg)
			if len(logs) == 2 {
				close(stop)
			}
		})
	}()

	err = <-done
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, logs)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
