package fleet

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/connect"
	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// The available device sources.
const (
	SourceMQTT = "mqtt"
	SourceHub  = "hub"
)

// An Announcement is returned by a backend when collecting devices.
//
// Note: Not correctly formatted announcements are ignored.
type Announcement struct {
	Source     string
	BaseTopic  string
	DeviceID   string
	DeviceName string
	AppName    string
	AppVersion string
}

// A Backend provides access to the devices of a fleet.
type Backend interface {
	// Collect will collect announcements from all available devices for the
	// specified duration.
	Collect(duration time.Duration) ([]*Announcement, error)

	// Resolve will return the addressable devices for the provided fleet
	// devices. The returned list is aligned with the provided list. Devices
	// that cannot be addressed yield a device that fails when opened.
	Resolve(devices []*Device) ([]msg.Device, error)

	// Close will close the backend and all underlying connections.
	Close()
}

type missingDevice struct {
	name string
	err  error
}

func (d *missingDevice) ID() string   { return d.name }
func (d *missingDevice) Type() string { return "Missing" }
func (d *missingDevice) Name() string { return d.name }

func (d *missingDevice) Open() (*msg.Channel, error) {
	return nil, d.err
}

type mqttBackend struct {
	router *mqtt.Router
}

// NewMQTTBackend will create a backend that connects to the specified MQTT
// broker.
func NewMQTTBackend(url string) (Backend, error) {
	// create router
	router, err := mqtt.Connect(url, "naos-fleet", packet.QOSAtMostOnce)
	if err != nil {
		return nil, err
	}

	return &mqttBackend{router: router}, nil
}

func (b *mqttBackend) Collect(duration time.Duration) ([]*Announcement, error) {
	// collect devices
	list, err := mqtt.Collect(b.router, duration)
	if err != nil {
		return nil, err
	}

	// prepare announcements
	var ans []*Announcement
	for _, a := range list {
		ans = append(ans, &Announcement{
			Source:     SourceMQTT,
			BaseTopic:  a.BaseTopic,
			DeviceName: a.DeviceName,
			AppName:    a.AppName,
			AppVersion: a.AppVersion,
		})
	}

	return ans, nil
}

func (b *mqttBackend) Resolve(devices []*Device) ([]msg.Device, error) {
	// create devices
	list := make([]msg.Device, 0, len(devices))
	for _, device := range devices {
		list = append(list, mqtt.NewDevice(b.router, device.BaseTopic))
	}

	return list, nil
}

func (b *mqttBackend) Close() {
	_ = b.router.Close()
}

type hubBackend struct {
	listURL string
	token   string
}

// NewHubBackend will create a backend that connects to the specified NAOS hub.
func NewHubBackend(base, token string) (Backend, error) {
	// prepare list URL
	listURL, err := url.JoinPath(base, "/list")
	if err != nil {
		return nil, err
	}

	return &hubBackend{listURL: listURL, token: token}, nil
}

// Note: The hub reports the connected devices immediately, therefore the
// duration is ignored.
func (b *hubBackend) Collect(time.Duration) ([]*Announcement, error) {
	// list devices
	list, err := b.list()
	if err != nil {
		return nil, err
	}

	// prepare announcements
	var ans []*Announcement
	for _, d := range list {
		// use device ID if the device is unnamed
		name := d.DeviceName
		if name == "" {
			name = d.DeviceID
		}

		// add announcement
		ans = append(ans, &Announcement{
			Source:     SourceHub,
			DeviceID:   d.DeviceID,
			DeviceName: name,
			AppName:    d.AppName,
			AppVersion: d.AppVersion,
		})
	}

	return ans, nil
}

func (b *hubBackend) Resolve(devices []*Device) ([]msg.Device, error) {
	// list devices
	list, err := b.list()
	if err != nil {
		return nil, err
	}

	// index descriptions by device ID
	index := make(map[string]connect.Description, len(list))
	for _, d := range list {
		index[d.DeviceID] = d
	}

	// create devices
	result := make([]msg.Device, 0, len(devices))
	for _, device := range devices {
		// get description
		desc, ok := index[device.DeviceID]
		if !ok {
			result = append(result, &missingDevice{
				name: device.DeviceName,
				err:  fmt.Errorf("device %q not connected to hub", device.DeviceName),
			})
			continue
		}

		// use hub token if the device has no dedicated token
		token := desc.AttachToken
		if token == "" {
			token = b.token
		}

		// add device
		result = append(result, connect.NewDevice(desc.AttachURL, token))
	}

	return result, nil
}

func (b *hubBackend) Close() {}

func (b *hubBackend) list() ([]connect.Description, error) {
	return connect.List(b.listURL, b.token)
}

type multiBackend struct {
	backends map[string]Backend
	fallback string
}

func (b *multiBackend) Collect(duration time.Duration) ([]*Announcement, error) {
	// collect from all backends
	var ans []*Announcement
	for _, backend := range b.backends {
		list, err := backend.Collect(duration)
		if err != nil {
			return nil, err
		}
		ans = append(ans, list...)
	}

	return ans, nil
}

func (b *multiBackend) Resolve(devices []*Device) ([]msg.Device, error) {
	// prepare result
	result := make([]msg.Device, len(devices))

	// group device indexes by source
	groups := make(map[string][]int)
	for i, device := range devices {
		// get source and ensure fallback
		source := device.Source
		if source == "" {
			source = b.fallback
		}

		// check backend
		if _, ok := b.backends[source]; !ok {
			result[i] = &missingDevice{
				name: device.DeviceName,
				err:  fmt.Errorf("no %q backend configured", source),
			}
			continue
		}

		// add index
		groups[source] = append(groups[source], i)
	}

	// resolve devices per source
	for source, indexes := range groups {
		// prepare subset
		subset := make([]*Device, 0, len(indexes))
		for _, i := range indexes {
			subset = append(subset, devices[i])
		}

		// resolve subset
		list, err := b.backends[source].Resolve(subset)
		if err != nil {
			return nil, err
		}

		// scatter results
		for j, i := range indexes {
			result[i] = list[j]
		}
	}

	return result, nil
}

func (b *multiBackend) Close() {
	for _, backend := range b.backends {
		backend.Close()
	}
}

// NewBackend will create a backend for the provided broker and hub. If both are
// specified, devices are resolved using their configured source.
func NewBackend(broker, hub, hubToken string) (Backend, error) {
	// prepare backends
	backends := make(map[string]Backend, 2)
	var fallback string

	// ensure cleanup on error
	var failed bool
	defer func() {
		if failed {
			for _, backend := range backends {
				backend.Close()
			}
		}
	}()

	// create MQTT backend
	if broker != "" {
		backend, err := NewMQTTBackend(broker)
		if err != nil {
			failed = true
			return nil, err
		}
		backends[SourceMQTT] = backend
		fallback = SourceMQTT
	}

	// create hub backend
	if hub != "" {
		backend, err := NewHubBackend(hub, hubToken)
		if err != nil {
			failed = true
			return nil, err
		}
		backends[SourceHub] = backend
		if fallback == "" {
			fallback = SourceHub
		}
	}

	// check backends
	if len(backends) == 0 {
		return nil, errors.New("no broker or hub configured")
	}

	return &multiBackend{backends: backends, fallback: fallback}, nil
}
