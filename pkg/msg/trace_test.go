package msg

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartTrace(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: traceEndpoint, Data: []byte{0}}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = StartTrace(s, time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestStopTrace(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: traceEndpoint, Data: []byte{1}}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = StopTrace(s, time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadTrace(t *testing.T) {
	var chunk []byte

	// TASK(7): TYPE(1) ID(1) NAME(*) NUL(1)
	chunk = append(chunk, 7, 2)
	chunk = append(chunk, []byte("my-task")...)
	chunk = append(chunk, 0)

	// LABEL(6): TYPE(1) ID(1) TEXT(*) NUL(1)
	chunk = append(chunk, 6, 0)
	chunk = append(chunk, []byte("app")...)
	chunk = append(chunk, 0)

	// LABEL(6): another label
	chunk = append(chunk, 6, 1)
	chunk = append(chunk, []byte("handle")...)
	chunk = append(chunk, 0)

	// SWITCH(1): TYPE(1) TS(4) CORE(1) ID(1) = 7
	ts := make([]byte, 4)
	binary.LittleEndian.PutUint32(ts, 1000)
	chunk = append(chunk, 1)
	chunk = append(chunk, ts...)
	chunk = append(chunk, 1) // core
	chunk = append(chunk, 2) // task id

	// EVENT(2): TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) = 9
	binary.LittleEndian.PutUint32(ts, 2000)
	chunk = append(chunk, 2)
	chunk = append(chunk, ts...)
	chunk = append(chunk, 0) // cat
	chunk = append(chunk, 1) // name
	arg := make([]byte, 2)
	binary.LittleEndian.PutUint16(arg, 42)
	chunk = append(chunk, arg...)

	// BEGIN(3): TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) ID(1) = 10
	binary.LittleEndian.PutUint32(ts, 3000)
	chunk = append(chunk, 3)
	chunk = append(chunk, ts...)
	chunk = append(chunk, 0)    // cat
	chunk = append(chunk, 1)    // name
	chunk = append(chunk, 0, 0) // arg
	chunk = append(chunk, 7)    // span id

	// END(4): TYPE(1) TS(4) ID(1) = 6
	binary.LittleEndian.PutUint32(ts, 4000)
	chunk = append(chunk, 4)
	chunk = append(chunk, ts...)
	chunk = append(chunk, 7) // span id

	// VALUE(5): TYPE(1) TS(4) CAT(1) NAME(1) VAL(4) = 11
	binary.LittleEndian.PutUint32(ts, 5000)
	chunk = append(chunk, 5)
	chunk = append(chunk, ts...)
	chunk = append(chunk, 0) // cat
	chunk = append(chunk, 1) // name label
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, uint32(0xFFFFFF9C)) // -100 as uint32
	chunk = append(chunk, val...)

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: traceEndpoint, Data: []byte{2}}),
		send(Message{Endpoint: traceEndpoint, Data: chunk}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	data, err := ReadTrace(s, time.Second)
	assert.NoError(t, err)

	// check task
	assert.Len(t, data.Tasks, 1)
	assert.Equal(t, uint8(2), data.Tasks[0].ID)
	assert.Equal(t, "my-task", data.Tasks[0].Name)

	// check labels
	assert.Len(t, data.Labels, 2)
	assert.Equal(t, uint8(0), data.Labels[0].ID)
	assert.Equal(t, "app", data.Labels[0].Text)
	assert.Equal(t, uint8(1), data.Labels[1].ID)
	assert.Equal(t, "handle", data.Labels[1].Text)

	// check events
	assert.Len(t, data.Events, 5)

	// SWITCH
	assert.Equal(t, TraceTaskSwitch, data.Events[0].Type)
	assert.Equal(t, uint32(1000), data.Events[0].Timestamp)
	assert.Equal(t, uint8(1), data.Events[0].Core)
	assert.Equal(t, uint8(2), data.Events[0].Task)

	// EVENT
	assert.Equal(t, TraceInstant, data.Events[1].Type)
	assert.Equal(t, uint32(2000), data.Events[1].Timestamp)
	assert.Equal(t, uint8(0), data.Events[1].Cat)
	assert.Equal(t, uint8(1), data.Events[1].Name)
	assert.Equal(t, uint16(42), data.Events[1].Arg)

	// BEGIN
	assert.Equal(t, TraceBegin, data.Events[2].Type)
	assert.Equal(t, uint32(3000), data.Events[2].Timestamp)
	assert.Equal(t, uint8(0), data.Events[2].Cat)
	assert.Equal(t, uint8(1), data.Events[2].Name)
	assert.Equal(t, uint8(7), data.Events[2].Span)

	// END
	assert.Equal(t, TraceEnd, data.Events[3].Type)
	assert.Equal(t, uint32(4000), data.Events[3].Timestamp)
	assert.Equal(t, uint8(7), data.Events[3].Span)

	// VALUE
	assert.Equal(t, TraceValue, data.Events[4].Type)
	assert.Equal(t, uint32(5000), data.Events[4].Timestamp)
	assert.Equal(t, uint8(0), data.Events[4].Cat)
	assert.Equal(t, uint8(1), data.Events[4].Name)
	assert.Equal(t, int32(-100), data.Events[4].Value)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadTraceEmpty(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: traceEndpoint, Data: []byte{2}}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	data, err := ReadTrace(s, time.Second)
	assert.NoError(t, err)
	assert.Empty(t, data.Tasks)
	assert.Empty(t, data.Labels)
	assert.Empty(t, data.Events)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestGetTraceStatus(t *testing.T) {
	// build status reply: ACTIVE(1) | BUF_SIZE(4) | BUF_USED(4) | DROPPED(4)
	status := make([]byte, 13)
	status[0] = 1 // active
	binary.LittleEndian.PutUint32(status[1:], 16384)
	binary.LittleEndian.PutUint32(status[5:], 100)
	binary.LittleEndian.PutUint32(status[9:], 3)

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: traceEndpoint, Data: []byte{3}}),
		send(Message{Endpoint: traceEndpoint, Data: status}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	st, err := GetTraceStatus(s, time.Second)
	assert.NoError(t, err)
	assert.True(t, st.Active)
	assert.Equal(t, uint32(16384), st.BufSize)
	assert.Equal(t, uint32(100), st.BufUsed)
	assert.Equal(t, uint32(3), st.Dropped)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
