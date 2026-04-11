package msg

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannelRoutesSessionLifecycle(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 4)
	channel.Subscribe(queue)

	openOwnedSession(t, channel, tr, queue, "open-happy", 21)

	dataMsg := Message{Session: 21, Endpoint: 0x42, Data: []byte("payload")}
	closeMsg := Message{Session: 21, Endpoint: 0xFF, Data: []byte{}}
	tr.reads <- dataMsg.Build()
	tr.reads <- closeMsg.Build()

	expectQueueMsg(t, queue, dataMsg)
	expectQueueMsg(t, queue, closeMsg)
	expectNoQueueMsg(t, queue)
}

func TestChannelRegistersPendingBeforeWriteReturns(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 4)
	channel.Subscribe(queue)

	handle := []byte("open-race")
	replyMsg := Message{Session: 21, Endpoint: 0x0, Data: handle}
	dataMsg := Message{Session: 21, Endpoint: 0x42, Data: []byte("payload")}

	tr.onWrite = func(written []byte) error {
		tr.reads <- replyMsg.Build()
		tr.reads <- dataMsg.Build()
		return nil
	}

	assert.NoError(t, channel.Write(queue, Message{Session: 0, Endpoint: 0x0, Data: handle}))

	expectQueueMsg(t, queue, replyMsg)
	expectQueueMsg(t, queue, dataMsg)
	expectNoQueueMsg(t, queue)
}

func TestChannelDropsSessionOwnershipAfterPeerClose(t *testing.T) {
	channel, tr := newTestChannel(t)

	q1 := make(Queue, 4)
	q2 := make(Queue, 4)
	channel.Subscribe(q1)
	channel.Subscribe(q2)

	openOwnedSession(t, channel, tr, q1, "open-1", 21)
	close1 := Message{Session: 21, Endpoint: 0xFF, Data: []byte{}}
	tr.reads <- close1.Build()

	expectQueueMsg(t, q1, close1)
	expectNoQueueMsg(t, q2)

	openOwnedSession(t, channel, tr, q2, "open-2", 21)
	data2 := Message{Session: 21, Endpoint: 0x42, Data: []byte("payload")}
	tr.reads <- data2.Build()

	expectQueueMsg(t, q2, data2)
	expectNoQueueMsg(t, q1)
}

func TestChannelWaitsBrieflyForOwnedQueueBackpressure(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 1)
	channel.Subscribe(queue)

	openOwnedSession(t, channel, tr, queue, "open-backpressure", 7)
	payloadMsg := Message{Session: 7, Endpoint: 0x42, Data: []byte("payload")}
	busyMsg := Message{Session: 7, Endpoint: 0x01, Data: []byte("busy")}

	queue <- busyMsg

	done := make(chan struct{})
	go func() {
		tr.reads <- payloadMsg.Build()
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	expectQueueMsg(t, queue, busyMsg)
	expectQueueMsg(t, queue, payloadMsg)

	select {
	case <-done:
	case <-time.After(time.Second):
		assert.Fail(t, "timed out waiting for dispatch")
	}
}

func TestChannelDropsInvalidFrames(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 1)
	channel.Subscribe(queue)

	tr.reads <- []byte{0xFF, 0x01, 0x02}

	expectNoQueueMsg(t, queue)
}

func TestChannelRejectsWrongOwnerWrites(t *testing.T) {
	channel, tr := newTestChannel(t)

	q1 := make(Queue, 2)
	q2 := make(Queue, 2)
	channel.Subscribe(q1)
	channel.Subscribe(q2)

	openOwnedSession(t, channel, tr, q1, "open-owner", 9)

	assert.ErrorIs(t, channel.Write(q2, Message{Session: 9, Endpoint: 0x42, Data: []byte("payload")}), ErrSessionWrongOwner)
}

func TestChannelRoutesMultipleSessions(t *testing.T) {
	channel, tr := newTestChannel(t)

	q1 := make(Queue, 2)
	q2 := make(Queue, 2)
	channel.Subscribe(q1)
	channel.Subscribe(q2)

	openOwnedSession(t, channel, tr, q1, "open-1", 11)
	openOwnedSession(t, channel, tr, q2, "open-2", 12)

	msg1 := Message{Session: 11, Endpoint: 0x10, Data: []byte("a")}
	msg2 := Message{Session: 12, Endpoint: 0x11, Data: []byte("b")}
	tr.reads <- msg1.Build()
	tr.reads <- msg2.Build()

	expectQueueMsg(t, q1, msg1)
	expectQueueMsg(t, q2, msg2)
	expectNoQueueMsg(t, q1)
	expectNoQueueMsg(t, q2)
}

func newTestChannel(t *testing.T) (*Channel, *memTransport) {
	t.Helper()

	tr := &memTransport{
		reads: make(chan []byte, 8),
	}
	channel := NewChannel(tr, nil, 10)
	t.Cleanup(channel.Close)

	return channel, tr
}

func openOwnedSession(t *testing.T, channel *Channel, tr *memTransport, queue Queue, handle string, session uint16) {
	t.Helper()

	rawHandle := []byte(handle)
	assert.NoError(t, channel.Write(queue, Message{Session: 0, Endpoint: 0x0, Data: rawHandle}))

	replyMsg := Message{Session: session, Endpoint: 0x0, Data: rawHandle}
	tr.reads <- replyMsg.Build()
	expectQueueMsg(t, queue, replyMsg)
}

func expectQueueMsg(t *testing.T, queue Queue, want Message) {
	t.Helper()

	select {
	case got := <-queue:
		assert.Equal(t, want, got)
	case <-time.After(time.Second):
		assert.Fail(t, "timed out waiting for queue message")
	}
}

func expectNoQueueMsg(t *testing.T, queue Queue) {
	t.Helper()

	select {
	case got := <-queue:
		assert.Failf(t, "unexpected queue message", "%v", got)
	case <-time.After(10 * time.Millisecond):
	}
}

type memTransport struct {
	reads   chan []byte
	onWrite func([]byte) error
}

func (t *memTransport) Read() ([]byte, error) {
	data, ok := <-t.reads
	if !ok {
		return nil, io.EOF
	}
	return data, nil
}

func (t *memTransport) Write(data []byte) error {
	if t.onWrite != nil {
		return t.onWrite(data)
	}
	return nil
}

func (t *memTransport) Close() {
	close(t.reads)
}
