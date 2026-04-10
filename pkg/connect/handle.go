package connect

import "net/http"

// Handle returns an HTTP handler that upgrades websocket requests and invokes handler.
func Handle(handler func(*WebSocketConn)) http.Handler {
	return HandleWith(nil, handler)
}

// HandleWith returns an HTTP handler that upgrades websocket requests and invokes handler.
func HandleWith(fallback http.Handler, handler func(*WebSocketConn)) http.Handler {
	upgrader := NewWebSocketUpgrader(fallback)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r)
		if err != nil || conn == nil {
			return
		}
		defer conn.Close()

		handler(conn)
	})
}
