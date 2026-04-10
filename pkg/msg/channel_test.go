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

	data := Pack("ohob", uint8(1), uint16(21), uint8(0x42), []byte("payload"))
	closeMsg := Pack("ohob", uint8(1), uint16(21), uint8(0xFF), []byte{})
	tr.reads <- data
	tr.reads <- closeMsg

	expectQueueData(t, queue, data)
	expectQueueData(t, queue, closeMsg)
	expectNoQueueData(t, queue)
}

func TestChannelRegistersPendingBeforeWriteReturns(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 4)
	channel.Subscribe(queue)

	handle := []byte("open-race")
	reply := Pack("ohob", uint8(1), uint16(21), uint8(0x0), handle)
	data := Pack("ohob", uint8(1), uint16(21), uint8(0x42), []byte("payload"))

	tr.onWrite = func(written []byte) error {
		tr.reads <- reply
		tr.reads <- data
		return nil
	}

	assert.NoError(t, channel.Write(queue, Pack("ohob", uint8(1), uint16(0), uint8(0x0), handle)))

	expectQueueData(t, queue, reply)
	expectQueueData(t, queue, data)
	expectNoQueueData(t, queue)
}

func TestChannelDropsSessionOwnershipAfterPeerClose(t *testing.T) {
	channel, tr := newTestChannel(t)

	q1 := make(Queue, 4)
	q2 := make(Queue, 4)
	channel.Subscribe(q1)
	channel.Subscribe(q2)

	openOwnedSession(t, channel, tr, q1, "open-1", 21)
	close1 := Pack("ohob", uint8(1), uint16(21), uint8(0xFF), []byte{})
	tr.reads <- close1

	expectQueueData(t, q1, close1)
	expectNoQueueData(t, q2)

	openOwnedSession(t, channel, tr, q2, "open-2", 21)
	data2 := Pack("ohob", uint8(1), uint16(21), uint8(0x42), []byte("payload"))
	tr.reads <- data2

	expectQueueData(t, q2, data2)
	expectNoQueueData(t, q1)
}

func TestChannelWaitsBrieflyForOwnedQueueBackpressure(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 1)
	channel.Subscribe(queue)

	openOwnedSession(t, channel, tr, queue, "open-backpressure", 7)
	payload := Pack("ohob", uint8(1), uint16(7), uint8(0x42), []byte("payload"))

	queue <- []byte("busy")

	done := make(chan struct{})
	go func() {
		tr.reads <- payload
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	expectQueueData(t, queue, []byte("busy"))
	expectQueueData(t, queue, payload)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dispatch")
	}
}

func TestChannelDropsInvalidFrames(t *testing.T) {
	channel, tr := newTestChannel(t)

	queue := make(Queue, 1)
	channel.Subscribe(queue)

	tr.reads <- []byte{0xFF, 0x01, 0x02}

	expectNoQueueData(t, queue)
}

func TestChannelRejectsWrongOwnerWrites(t *testing.T) {
	channel, tr := newTestChannel(t)

	q1 := make(Queue, 2)
	q2 := make(Queue, 2)
	channel.Subscribe(q1)
	channel.Subscribe(q2)

	openOwnedSession(t, channel, tr, q1, "open-owner", 9)

	assert.ErrorIs(t, channel.Write(q2, Pack("ohob", uint8(1), uint16(9), uint8(0x42), []byte("payload"))), SessionWrongOwner)
}

func TestChannelRoutesMultipleSessions(t *testing.T) {
	channel, tr := newTestChannel(t)

	q1 := make(Queue, 2)
	q2 := make(Queue, 2)
	channel.Subscribe(q1)
	channel.Subscribe(q2)

	openOwnedSession(t, channel, tr, q1, "open-1", 11)
	openOwnedSession(t, channel, tr, q2, "open-2", 12)

	data1 := Pack("ohob", uint8(1), uint16(11), uint8(0x10), []byte("a"))
	data2 := Pack("ohob", uint8(1), uint16(12), uint8(0x11), []byte("b"))
	tr.reads <- data1
	tr.reads <- data2

	expectQueueData(t, q1, data1)
	expectQueueData(t, q2, data2)
	expectNoQueueData(t, q1)
	expectNoQueueData(t, q2)
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

func openOwnedSession(t *testing.T, channel *Channel, tr *memTransport, queue Queue, handle string, session uint16) []byte {
	t.Helper()

	rawHandle := []byte(handle)
	assert.NoError(t, channel.Write(queue, Pack("ohob", uint8(1), uint16(0), uint8(0x0), rawHandle)))

	reply := Pack("ohob", uint8(1), session, uint8(0x0), rawHandle)
	tr.reads <- reply
	expectQueueData(t, queue, reply)

	return reply
}

func expectQueueData(t *testing.T, queue Queue, want []byte) {
	t.Helper()

	select {
	case got := <-queue:
		assert.Equal(t, want, got)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queue payload")
	}
}

func expectNoQueueData(t *testing.T, queue Queue) {
	t.Helper()

	select {
	case got := <-queue:
		t.Fatalf("unexpected queue payload: %v", got)
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
