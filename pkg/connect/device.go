package connect

import (
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"

	"github.com/256dpi/naos/pkg/msg"
)

type device struct {
	baseURL string
	token   string
	id      string
	label   string
}

// NewDevice returns a msg.Device that connects through a Connect server.
func NewDevice(baseURL string, token string, id string) msg.Device {
	// determine label
	label := id
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Host != "" {
		label = parsed.Host + "/" + id
	}

	return &device{
		baseURL: baseURL,
		token:   token,
		id:      id,
		label:   label,
	}
}

func (d *device) ID() string {
	return "connect/" + d.label
}

func (d *device) Open() (*msg.Channel, error) {
	// get device URL
	target, err := deviceWebSocketURL(d.baseURL, d.id)
	if err != nil {
		return nil, err
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
