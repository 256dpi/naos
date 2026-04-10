package connect

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketUpgrader upgrades HTTP requests to the NAOS Connect websocket transport.
type WebSocketUpgrader struct {
	fallback http.Handler
	upgrader *websocket.Upgrader
}

// NewWebSocketUpgrader returns a websocket upgrader with NAOS Connect defaults.
func NewWebSocketUpgrader(fallback http.Handler) *WebSocketUpgrader {
	return &WebSocketUpgrader{
		fallback: fallback,
		upgrader: &websocket.Upgrader{
			HandshakeTimeout:  60 * time.Second,
			ReadBufferSize:    0,
			WriteBufferSize:   0,
			Subprotocols:      []string{"naos"},
			CheckOrigin:       func(*http.Request) bool { return true },
			EnableCompression: false,
		},
	}
}

// Upgrade upgrades a websocket request or forwards to the fallback handler.
func (u *WebSocketUpgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*WebSocketConn, error) {
	if !websocket.IsWebSocketUpgrade(r) {
		if u.fallback == nil {
			http.Error(w, "websocket upgrade required", http.StatusUpgradeRequired)
			return nil, nil
		}
		u.fallback.ServeHTTP(w, r)
		return nil, nil
	}

	conn, err := u.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	return NewWebSocketConn(conn), nil
}

// UnderlyingUpgrader returns the wrapped Gorilla websocket upgrader.
func (u *WebSocketUpgrader) UnderlyingUpgrader() *websocket.Upgrader {
	return u.upgrader
}
