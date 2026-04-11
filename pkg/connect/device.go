package connect

import (
	"fmt"

	"github.com/gorilla/websocket"

	"github.com/256dpi/naos/pkg/msg"
)

type device struct {
	url   string
	token string
	uuid  string
}

// NewDevice returns a msg.Device that connects through a Connect server.
func NewDevice(url string, token string, uuid string) msg.Device {
	return &device{
		url:   url,
		token: token,
		uuid:  uuid,
	}
}

func (d *device) ID() string {
	return "connect/" + d.uuid
}

func (d *device) Open() (*msg.Channel, error) {
	// check attach URL
	if d.url == "" {
		return nil, fmt.Errorf("missing attach url")
	}

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
	conn, resp, err := dialer.Dial(d.url, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial %s failed: %s", d.url, resp.Status)
		}
		return nil, err
	}

	// create channel
	ch := msg.NewChannel(NewTransport(NewWebSocketConn(conn)), d, 10)

	return ch, nil
}
