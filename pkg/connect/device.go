package connect

import (
	"fmt"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/256dpi/naos/pkg/msg"
)

type device struct {
	baseURL string
	token   string
	uuid    string
}

// NewDevice returns a msg.Device that connects through a Connect server.
func NewDevice(baseURL string, token string, uuid string) msg.Device {
	return &device{
		baseURL: baseURL,
		token:   token,
		uuid:    uuid,
	}
}

func (d *device) ID() string {
	return "connect/" + d.uuid
}

func (d *device) Open() (*msg.Channel, error) {
	// get device URL
	baseURL := strings.Replace(d.baseURL, "http://", "ws://", 1)
	target := strings.Join([]string{baseURL, d.uuid}, "/device/")

	// prepare dialer
	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{"naos"}

	// prepare header
	var header map[string][]string
	if d.token != "" {
		header = map[string][]string{
			"Authorization": {"Bearer " + d.token},
		}
	}

	// dial server
	conn, resp, err := dialer.Dial(target, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial %s failed: %s", target, resp.Status)
		}
		return nil, err
	}

	// create channel
	ch := msg.NewChannel(NewTransport(NewWebSocketConn(conn)), d, 10)

	return ch, nil
}
