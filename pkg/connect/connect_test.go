package connect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

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
			serverErrs <- assert.AnError
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
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// send a valid msg-protocol frame: version=1, session=0, endpoint=0x42, data={0xaa,0xbb}
	payload := msg.Pack("ohob", uint8(1), uint16(0), uint8(0x42), []byte{0xaa, 0xbb})
	err = conn.WriteMessage(websocket.BinaryMessage, append([]byte{version, cmdMsg}, payload...))
	assert.NoError(t, err)

	select {
	case m := <-payloads:
		assert.Equal(t, payload, m.Build())
	case <-time.After(time.Second):
		assert.Fail(t, "timed out waiting for payload")
	}

	err = conn.WriteMessage(websocket.BinaryMessage, []byte{0xff, cmdMsg, 0x01})
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		assert.Fail(t, "timed out waiting for channel shutdown")
	}

	_, _, err = conn.ReadMessage()
	assert.Error(t, err)

	select {
	case err := <-serverErrs:
		assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Nil(t, conn)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	select {
	case <-called:
	default:
		assert.Fail(t, "expected fallback handler to be called")
	}
}

func TestServerTokenAuthorization(t *testing.T) {
	server := NewServer()
	server.SetToken("secret")

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/list?token=secret", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/list", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/list", nil)
	req.Header.Set("Authorization", "secret")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/list", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"devices":[{"uuid":"device-1","device_id":"device-1","connected":"2026-04-11T12:00:00Z","attach_url":"wss://example.com/attach/1","attach_token":"attach-secret"}]}`))
	}))
	defer server.Close()

	devices, err := List(server.URL+"/list", "secret")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, devices, 1) {
		return
	}
	assert.Equal(t, "device-1", devices[0].DeviceID)
	assert.Equal(t, "device-1", devices[0].UUID)
	assert.Equal(t, "wss://example.com/attach/1", devices[0].AttachURL)
	assert.Equal(t, "attach-secret", devices[0].AttachToken)
}

func TestServerListEmptyDevices(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var out struct {
		Devices []Description `json:"devices"`
	}
	err := json.NewDecoder(rec.Body).Decode(&out)
	assert.NoError(t, err)
	assert.NotNil(t, out.Devices)
	assert.Empty(t, out.Devices)
}

func TestDeviceOpenUsesAttach(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer attach-secret", r.Header.Get("Authorization"))

		upgrader := websocket.Upgrader{
			Subprotocols: []string{"naos"},
			CheckOrigin:  func(*http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.NoError(t, err)
		close(done)
		_ = conn.Close()
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http:", "ws:", 1)
	dev := NewDevice(wsURL, "attach-secret")
	assert.Equal(t, "connect/"+strings.TrimPrefix(wsURL, "ws://"), dev.ID())

	ch, err := dev.Open()
	if !assert.NoError(t, err) {
		return
	}
	defer ch.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		assert.Fail(t, "timed out waiting for attach")
	}
}
