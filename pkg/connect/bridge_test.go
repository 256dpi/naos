package connect

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

func TestBridge(t *testing.T) {
	deviceConn := newMemoryConn()
	channel := msg.NewChannel(NewTransport(deviceConn), nil, 10)

	clientAConn := newMemoryConn()
	clientA := NewTransport(clientAConn)
	clientBConn := newMemoryConn()
	clientB := NewTransport(clientBConn)

	errs := make(chan error, 2)
	go func() {
		errs <- Bridge(clientA, channel)
	}()
	go func() {
		errs <- Bridge(clientB, channel)
	}()

	time.Sleep(50 * time.Millisecond)

	handleA := []byte("open-a")
	sendFrame(t, clientAConn.reads, rawMessage(0, 0x0, handleA))
	expectFrame(t, deviceConn.writes, rawMessage(0, 0x0, handleA))

	handleB := []byte("open-b")
	sendFrame(t, clientBConn.reads, rawMessage(0, 0x0, handleB))
	expectFrame(t, deviceConn.writes, rawMessage(0, 0x0, handleB))

	replyA := rawMessage(11, 0x0, handleA)
	sendFrame(t, deviceConn.reads, replyA)
	expectFrame(t, clientAConn.writes, replyA)
	expectNoFrame(t, clientBConn.writes)

	replyB := rawMessage(12, 0x0, handleB)
	sendFrame(t, deviceConn.reads, replyB)
	expectFrame(t, clientBConn.writes, replyB)
	expectNoFrame(t, clientAConn.writes)

	dataA := rawMessage(11, 0x22, []byte("from-device-a"))
	sendFrame(t, deviceConn.reads, dataA)
	expectFrame(t, clientAConn.writes, dataA)
	expectNoFrame(t, clientBConn.writes)

	dataB := rawMessage(12, 0x23, []byte("from-device-b"))
	sendFrame(t, deviceConn.reads, dataB)
	expectFrame(t, clientBConn.writes, dataB)
	expectNoFrame(t, clientAConn.writes)

	// client→device session data
	clientDataA := rawMessage(11, 0x30, []byte("from-client-a"))
	sendFrame(t, clientAConn.reads, clientDataA)
	expectFrame(t, deviceConn.writes, clientDataA)

	clientDataB := rawMessage(12, 0x31, []byte("from-client-b"))
	sendFrame(t, clientBConn.reads, clientDataB)
	expectFrame(t, deviceConn.writes, clientDataB)

	clientA.Close()
	clientB.Close()
	channel.Close()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil && err != io.EOF {
				t.Fatalf("unexpected bridge error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for bridge shutdown")
		}
	}
}

type memoryConn struct {
	reads  chan []byte
	writes chan []byte
	done   chan struct{}
	once   sync.Once
}

func newMemoryConn() *memoryConn {
	return &memoryConn{
		reads:  make(chan []byte, 16),
		writes: make(chan []byte, 16),
		done:   make(chan struct{}),
	}
}

func (c *memoryConn) LocalAddr() net.Addr {
	return memoryAddr("local")
}

func (c *memoryConn) RemoteAddr() net.Addr {
	return memoryAddr("remote")
}

func (c *memoryConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *memoryConn) Read() ([]byte, error) {
	select {
	case <-c.done:
		return nil, io.EOF
	case data := <-c.reads:
		return append([]byte(nil), data...), nil
	}
}

func (c *memoryConn) Write(data []byte) (int, error) {
	select {
	case <-c.done:
		return 0, io.EOF
	case c.writes <- append([]byte(nil), data...):
		return len(data), nil
	}
}

func (c *memoryConn) Close() error {
	c.once.Do(func() {
		close(c.done)
	})
	return nil
}

type memoryAddr string

func (a memoryAddr) Network() string {
	return "memory"
}

func (a memoryAddr) String() string {
	return string(a)
}

func rawMessage(session uint16, endpoint uint8, data []byte) []byte {
	return msg.Pack("ohob", uint8(1), session, endpoint, data)
}

func sendFrame(t *testing.T, ch chan<- []byte, payload []byte) {
	t.Helper()

	frame := append([]byte{version, cmdMsg}, payload...)
	select {
	case ch <- frame:
	case <-time.After(time.Second):
		t.Fatal("timed out sending frame")
	}
}

func expectFrame(t *testing.T, ch <-chan []byte, payload []byte) {
	t.Helper()

	select {
	case frame := <-ch:
		want := append([]byte{version, cmdMsg}, payload...)
		if string(frame) != string(want) {
			t.Fatalf("unexpected frame: got %v want %v", frame, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for frame")
	}
}

func expectNoFrame(t *testing.T, ch <-chan []byte) {
	t.Helper()

	select {
	case frame := <-ch:
		t.Fatalf("unexpected frame: %v", frame)
	case <-time.After(50 * time.Millisecond):
	}
}
