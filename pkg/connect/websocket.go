package connect

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ErrNotBinary indicates that a non-binary websocket frame was received.
var ErrNotBinary = errors.New("received non-binary web socket message")

// WebSocketConn adapts a Gorilla websocket connection to Conn.
type WebSocketConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewWebSocketConn wraps a Gorilla websocket connection.
func NewWebSocketConn(conn *websocket.Conn) *WebSocketConn {
	return &WebSocketConn{
		conn: conn,
	}
}

// LocalAddr returns the local network address.
func (c *WebSocketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *WebSocketConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetReadDeadline forwards the read deadline to the websocket connection.
func (c *WebSocketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// Read returns the next binary websocket message.
func (c *WebSocketConn) Read() ([]byte, error) {
	// read next message
	messageType, buf, err := c.conn.ReadMessage()
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		return nil, io.EOF
	} else if err != nil {
		return nil, err
	}

	// check message type
	if messageType != websocket.BinaryMessage {
		return nil, ErrNotBinary
	}

	return buf, nil
}

// Write sends a single binary websocket message.
func (c *WebSocketConn) Write(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// get writer
	writer, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}

	// write buffer
	n, err := writer.Write(buf)
	if err != nil {
		return n, err
	}

	// close writer
	err = writer.Close()
	if err != nil {
		return n, err
	}

	return n, nil
}

// Close closes the websocket connection.
func (c *WebSocketConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.Close()
}
