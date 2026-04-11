package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: 0xFE}),
		send(Message{Endpoint: 0xFE, Data: []byte{1}}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = s.Ping(time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestQuery(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint}),
		send(Message{Endpoint: 0xFE, Data: []byte{1}}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	ok, err := s.Query(paramsEndpoint, time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestStatus(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: SystemEndpoint, Data: Pack("o", uint8(0))}),
		send(Message{Endpoint: SystemEndpoint, Data: []byte{0}}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	status, err := s.Status(time.Second)
	assert.NoError(t, err)
	assert.Equal(t, Status(0), status)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestUnlock(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: SystemEndpoint, Data: Pack("os", uint8(1), "secret")}),
		send(Message{Endpoint: SystemEndpoint, Data: []byte{1}}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	ok, err := s.Unlock("secret", time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestGetMTU(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: SystemEndpoint, Data: Pack("o", uint8(2))}),
		send(Message{Endpoint: SystemEndpoint, Data: Pack("h", uint16(512))}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	mtu, err := s.GetMTU(time.Second)
	assert.NoError(t, err)
	assert.Equal(t, uint16(512), mtu)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
