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

// NewDevice returns a msg.Device that connects through a NAOS Connect server.
func NewDevice(baseURL string, id string) msg.Device {
	return NewDeviceWithToken(baseURL, "", id)
}

// NewDeviceWithToken returns a msg.Device that connects through a protected NAOS Connect server.
func NewDeviceWithToken(baseURL string, token string, id string) msg.Device {
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

func (d *device) Open() (msg.Channel, error) {
	target, err := deviceWebSocketURL(d.baseURL, d.id)
	if err != nil {
		return nil, err
	}

	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{"naos"}

	var header map[string][]string
	if d.token != "" {
		header = map[string][]string{
			"Authorization": {"Bearer " + d.token},
		}
	}

	conn, resp, err := dialer.Dial(target, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial %s failed: %s", target, resp.Status)
		}
		return nil, err
	}

	return NewChannel(d, NewWebSocketConn(conn)), nil
}
