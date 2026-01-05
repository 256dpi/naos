package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/gomqtt/packet"
	"github.com/samber/lo"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// A LogMessage receive from a device.
type LogMessage struct {
	Time      time.Time
	BaseTopic string
	Content   string
}

// Record will enable log recording mode on all devices and yield the received
// log messages until the provided channel has been closed.
func Record(url string, baseTopics []string, stop chan struct{}, cb func(*LogMessage)) error {
	// check base topics
	if len(baseTopics) == 0 {
		return errors.New("zero base topics")
	}

	// create router
	router, err := mqtt.Connect(url, "naos-fleet", packet.QOSAtMostOnce)
	if err != nil {
		return err
	}
	defer router.Close()

	// create devices
	devices := make([]msg.Device, 0, len(baseTopics))
	for _, baseTopic := range baseTopics {
		devices = append(devices, mqtt.NewDevice(router, baseTopic))
	}

	// execute log streaming
	results := msg.Execute(devices, len(devices), func(s *msg.Session) (any, error) {
		// get index and base topic
		index := lo.IndexOf(devices, s.Channel().Device())
		baseTopic := baseTopics[index]

		return nil, msg.StreamLog(s, stop, func(content string) {
			// call callback
			cb(&LogMessage{
				Time:      time.Now(),
				BaseTopic: baseTopic,
				Content:   content,
			})
		})
	})
	for _, result := range results {
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}
