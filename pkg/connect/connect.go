package connect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Description describes a remotely connected device.
type Description struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"device_id,omitempty"`
	DeviceName string    `json:"device_name,omitempty"`
	AppName    string    `json:"app_name,omitempty"`
	AppVersion string    `json:"app_version,omitempty"`
	Connected  time.Time `json:"connected"`
}

// List fetches the currently connected devices from a NAOS Connect server.
func List(rawURL string) ([]Description, error) {
	base, err := normalizeBaseURL(rawURL)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(base)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var devices []Description
	err = json.NewDecoder(resp.Body).Decode(&devices)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

func normalizeBaseURL(rawURL string) (string, error) {
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	if u.Host == "" && u.Path != "" {
		u.Host = u.Path
		u.Path = ""
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid connect URL")
	}

	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/"
	}
	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

func deviceWebSocketURL(baseURL string, id string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported connect scheme: %s", u.Scheme)
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/device/" + url.PathEscape(id)

	return u.String(), nil
}
