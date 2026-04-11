package connect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Description describes a remotely connected device.
type Description struct {
	UUID       string    `json:"uuid"`
	DeviceID   string    `json:"device_id,omitempty"`
	DeviceName string    `json:"device_name,omitempty"`
	AppName    string    `json:"app_name,omitempty"`
	AppVersion string    `json:"app_version,omitempty"`
	Connected  time.Time `json:"connected"`
}

// List fetches the currently connected devices from a NAOS Connect server.
func List(baseURL string, token string) ([]Description, error) {
	// prepare request
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
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
	var devices []Description
	err = json.NewDecoder(resp.Body).Decode(&devices)
	if err != nil {
		return nil, err
	}

	return devices, nil
}
