package connect

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/256dpi/naos/pkg/msg"
)

func TestConnect(t *testing.T) {
	payloads := make(chan []byte, 1)
	done := make(chan struct{})
	serverErrs := make(chan error, 1)

	server := httptest.NewServer(Handle(func(conn *WebSocketConn) {
		channel := NewChannel(nil, conn)
		queue := make(msg.Queue, 1)
		channel.Subscribe(queue)
		defer channel.Unsubscribe(queue)

		select {
		case data := <-queue:
			payloads <- data
		case <-time.After(time.Second):
			serverErrs <- io.ErrNoProgress
			return
		}

		<-channel.Done()
		close(done)
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):]
	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{"naos"}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	err = conn.WriteMessage(websocket.BinaryMessage, []byte{version, cmdMsg, 0xaa, 0xbb})
	if err != nil {
		t.Fatalf("failed to write valid frame: %v", err)
	}

	select {
	case data := <-payloads:
		if len(data) != 2 || data[0] != 0xaa || data[1] != 0xbb {
			t.Fatalf("unexpected payload: %v", data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for payload")
	}

	err = conn.WriteMessage(websocket.BinaryMessage, []byte{0xff, cmdMsg, 0x01})
	if err != nil {
		t.Fatalf("failed to write invalid frame: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel shutdown")
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected client connection to close after protocol error")
	}

	select {
	case err := <-serverErrs:
		if err != nil {
			t.Fatalf("server handler failed: %v", err)
		}
	default:
	}
}

func TestWebSocketUpgraderFallback(t *testing.T) {
	called := make(chan struct{}, 1)
	upgrader := NewWebSocketUpgrader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	conn, err := upgrader.Upgrade(rec, req)
	if err != nil {
		t.Fatalf("unexpected upgrade error: %v", err)
	}
	if conn != nil {
		t.Fatal("expected no websocket connection for fallback request")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected fallback status: got %d want %d", rec.Code, http.StatusNoContent)
	}
	select {
	case <-called:
	default:
		t.Fatal("expected fallback handler to be called")
	}
}
