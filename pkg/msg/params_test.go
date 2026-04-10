package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetParam(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: Pack("os", uint8(0), "foo")}),
		send(Message{Endpoint: paramsEndpoint, Data: []byte("bar")}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	val, err := GetParam(s, "foo", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), val)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestSetParam(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: Pack("osob", uint8(1), "foo", uint8(0), []byte("bar"))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = SetParam(s, "foo", []byte("bar"), time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestListParams(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: []byte{2}}),
		send(Message{Endpoint: paramsEndpoint, Data: Pack("ooob", uint8(1), uint8(ParamTypeString), uint8(ParamModeApplication), []byte("foo"))}),
		send(Message{Endpoint: paramsEndpoint, Data: Pack("ooob", uint8(2), uint8(ParamTypeLong), uint8(ParamModeSystem), []byte("bar"))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	list, err := ListParams(s, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []ParamInfo{
		{Ref: 1, Type: ParamTypeString, Mode: ParamModeApplication, Name: "foo"},
		{Ref: 2, Type: ParamTypeLong, Mode: ParamModeSystem, Name: "bar"},
	}, list)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadParam(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: []byte{3, 7}}),
		send(Message{Endpoint: paramsEndpoint, Data: []byte("hello")}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	val, err := ReadParam(s, 7, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), val)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestWriteParam(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: Pack("oob", uint8(4), uint8(7), []byte("hello"))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = WriteParam(s, 7, []byte("hello"), time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestCollectParams(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: Pack("oqq", uint8(5), uint64((1<<1)|(1<<3)), uint64(100))}),
		send(Message{Endpoint: paramsEndpoint, Data: Pack("oqb", uint8(1), uint64(200), []byte("val1"))}),
		send(Message{Endpoint: paramsEndpoint, Data: Pack("oqb", uint8(3), uint64(300), []byte("val2"))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	updates, err := CollectParams(s, []uint8{1, 3}, 100, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []ParamUpdate{
		{Ref: 1, Age: 200, Value: []byte("val1")},
		{Ref: 3, Age: 300, Value: []byte("val2")},
	}, updates)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestClearParam(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: paramsEndpoint, Data: []byte{6, 7}}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = ClearParam(s, 7, time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
