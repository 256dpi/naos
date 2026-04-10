package msg

import (
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testMessage represents a single message in a test device script.
type testMessage struct {
	msg      Message
	incoming bool // true = device sends this to client
}

// send creates a test message sent by the device to the client.
func send(msg Message) testMessage {
	return testMessage{msg: msg, incoming: true}
}

// receive creates a test message expected to be received from the client.
func receive(msg Message) testMessage {
	return testMessage{msg: msg, incoming: false}
}

// ack creates an acknowledgement message sent by the device.
func ack() testMessage {
	return send(Message{Endpoint: 0xFE, Data: []byte{1}})
}

// testDevice implements the Device interface using a scripted message exchange.
type testDevice struct {
	channel *Channel
}

func (d *testDevice) ID() string {
	return "test"
}

func (d *testDevice) Open() (*Channel, error) {
	return d.channel, nil
}

// newTestDevice creates a test device that replays the given message script.
// Session open and close are handled automatically. The session ID is set on
// all messages automatically.
func newTestDevice(t *testing.T, session uint16, messages []testMessage) *testDevice {
	t.Helper()

	// set session ID and normalize nil data
	for i := range messages {
		messages[i].msg.Session = session
		if messages[i].msg.Data == nil {
			messages[i].msg.Data = []byte{}
		}
	}

	tr := &scriptTransport{
		reads: make(chan []byte),
		writes: make(chan []byte),
		stop:   make(chan struct{}),
	}

	ch := NewChannel(tr, nil, 10)

	done := make(chan struct{})
	go func() {
		defer close(done)
		runScript(t, tr, session, messages)
	}()

	t.Cleanup(func() {
		ch.Close()
		<-done
	})

	return &testDevice{channel: ch}
}

func runScript(t *testing.T, tr *scriptTransport, session uint16, messages []testMessage) {
	// handle session open
	data, ok := tr.awaitWrite()
	if !ok {
		return
	}
	open, ok := Parse(data)
	if !ok || open.Session != 0 || open.Endpoint != 0x0 {
		t.Errorf("test device: expected valid session open")
		tr.Close()
		return
	}
	reply := Message{Session: session, Endpoint: 0x0, Data: open.Data}
	if !tr.feedRead(reply.Build()) {
		return
	}

	// process message script
	for i, tm := range messages {
		if tm.incoming {
			if !tr.feedRead(tm.msg.Build()) {
				return
			}
		} else {
			data, ok := tr.awaitWrite()
			if !ok {
				return
			}
			got, ok := Parse(data)
			if !ok {
				t.Errorf("test device: message %d: failed to parse", i)
				tr.Close()
				return
			}
			assert.Equal(t, tm.msg.Session, got.Session, "test device: message %d: session", i)
			assert.Equal(t, tm.msg.Endpoint, got.Endpoint, "test device: message %d: endpoint", i)
			assert.Equal(t, tm.msg.Data, got.Data, "test device: message %d: data", i)
		}
	}

	// handle session close
	data, ok = tr.awaitWrite()
	if !ok {
		return
	}
	end, ok := Parse(data)
	if !ok || end.Session != session || end.Endpoint != 0xFF {
		t.Errorf("test device: expected valid session close")
		tr.Close()
		return
	}
	closeReply := Message{Session: session, Endpoint: 0xFF, Data: []byte{}}
	tr.feedRead(closeReply.Build())
}

// scriptTransport is a Transport that synchronizes with a control goroutine.
type scriptTransport struct {
	reads    chan []byte // control goroutine sends, Read() receives
	writes   chan []byte // Write() sends, control goroutine receives
	stop     chan struct{}
	stopOnce sync.Once
}

func (tr *scriptTransport) Read() ([]byte, error) {
	select {
	case data := <-tr.reads:
		return data, nil
	case <-tr.stop:
		return nil, io.EOF
	}
}

func (tr *scriptTransport) Write(data []byte) error {
	select {
	case tr.writes <- data:
		return nil
	case <-tr.stop:
		return io.EOF
	}
}

func (tr *scriptTransport) Close() {
	tr.stopOnce.Do(func() {
		close(tr.stop)
	})
}

func (tr *scriptTransport) awaitWrite() ([]byte, bool) {
	select {
	case data := <-tr.writes:
		return data, true
	case <-tr.stop:
		return nil, false
	}
}

func (tr *scriptTransport) feedRead(data []byte) bool {
	select {
	case tr.reads <- data:
		return true
	case <-tr.stop:
		return false
	}
}
