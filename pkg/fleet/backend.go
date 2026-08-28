package fleet

import (
	"time"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// An Announcement is returned by a backend when collecting devices.
//
// Note: Not correctly formatted announcements are ignored.
type Announcement struct {
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
