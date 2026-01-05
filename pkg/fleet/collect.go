package fleet

import (
	"time"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/mqtt"
)

// An Announcement is returned by Collect.
type Announcement struct {
	BaseTopic  string
	DeviceName string
	AppName    string
	AppVersion string
}

// Collect will collect Announcements from devices by connecting to the provided
// MQTT broker and sending the 'collect' command.
//
// Note: Not correctly formatted announcements are ignored.
func Collect(url string, duration time.Duration) ([]*Announcement, error) {
	// create router
	router, err := mqtt.Connect(url, "naos-fleet", packet.QOSAtMostOnce)
	if err != nil {
		return nil, err
	}
	defer router.Close()

	// collect devices
	list, err := mqtt.Collect(router, duration)
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
