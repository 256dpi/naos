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
	payloads := make(chan msg.Message, 1)
	done := make(chan struct{})
	serverErrs := make(chan error, 1)

	server := httptest.NewServer(Handle(func(conn *WebSocketConn) {
		channel := msg.NewChannel(NewTransport(conn), nil, 10)
		queue := make(msg.Queue, 1)
		channel.Subscribe(queue)
		defer channel.Unsubscribe(queue)

		select {
		case m := <-queue:
			payloads <- m
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

	// send a valid msg-protocol frame: version=1, session=0, endpoint=0x42, data={0xaa,0xbb}
	payload := msg.Pack("ohob", uint8(1), uint16(0), uint8(0x42), []byte{0xaa, 0xbb})
	err = conn.WriteMessage(websocket.BinaryMessage, append([]byte{version, cmdMsg}, payload...))
	if err != nil {
		t.Fatalf("failed to write valid frame: %v", err)
	}

	select {
	case m := <-payloads:
		if string(m.Build()) != string(payload) {
			t.Fatalf("unexpected payload: %v", m)
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

func TestServerTokenAuthorization(t *testing.T) {
	server := NewServer()
	server.SetToken("secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status without token: got %d want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/?token=secret", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status with query token: got %d want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status with bearer token: got %d want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "secret")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status with raw token: got %d want %d", rec.Code, http.StatusOK)
	}
}

func TestList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	devices, err := List(server.URL, "secret")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("unexpected devices: %v", devices)
	}
}
