package connect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Description describes a remotely connected device.
type Description struct {
	UUID        string    `json:"uuid"`
	Connected   time.Time `json:"connected"`
	DeviceID    string    `json:"device_id,omitempty"`
	DeviceName  string    `json:"device_name,omitempty"`
	AppName     string    `json:"app_name,omitempty"`
	AppVersion  string    `json:"app_version,omitempty"`
	AttachURL   string    `json:"attach_url"`
	AttachToken string    `json:"attach_token,omitempty"`
}

type listResponse struct {
	Devices []Description `json:"devices"`
}

// List fetches the currently connected devices from a NAOS Connect server.
func List(url string, token string) ([]Description, error) {
	// prepare request
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// perform request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// parse descriptions
	var out listResponse
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, err
	}

	return out.Devices, nil
}
