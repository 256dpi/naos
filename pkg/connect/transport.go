package connect

import (
	"net"
	"time"
)

const (
	version = 0x1
	cmdMsg  = 0x0
)

// Conn exchanges packet-oriented binary messages over a connection.
type Conn interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetReadDeadline(time.Time) error
	Read() ([]byte, error)
	Write([]byte) (int, error)
	Close() error
}

// Transport adapts a connection to the msg.Transport interface.
type Transport struct {
	conn Conn
}

// NewTransport wraps the provided connection in a raw transport.
func NewTransport(conn Conn) *Transport {
	return &Transport{
		conn: conn,
	}
}

func (t *Transport) Read() ([]byte, error) {
	// read message
	buf, err := t.conn.Read()
	if err != nil {
		return nil, err
	}

	// check header
	if len(buf) < 2 || buf[0] != version || buf[1] != cmdMsg {
		_ = t.conn.Close()
		return nil, net.ErrClosed
	}

	// extract message
	msg := append([]byte(nil), buf[2:]...)

	return msg, nil
}

func (t *Transport) Write(data []byte) error {
	// frame message
	buf := make([]byte, 2+len(data))
	buf[0] = version
	buf[1] = cmdMsg
	copy(buf[2:], data)

	// write message
	_, err := t.conn.Write(buf)
	return err
}

func (t *Transport) Close() {
	// close connection
	_ = t.conn.Close()
}
