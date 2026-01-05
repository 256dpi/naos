package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// Debug will request coredump debug information from the specified devices.
func Debug(url string, baseTopics []string, delete bool, jobs int) (map[string][]byte, error) {
	// check base topics
	if len(baseTopics) == 0 {
		return nil, errors.New("zero base topics")
	}

	// create router
	router, err := mqtt.Connect(url, "naos-fleet", packet.QOSAtMostOnce)
	if err != nil {
		return nil, err
	}
	defer router.Close()

	// create devices
	devices := make([]msg.Device, 0, len(baseTopics))
	for _, baseTopic := range baseTopics {
		devices = append(devices, mqtt.NewDevice(router, baseTopic))
	}

	// execute
	results := msg.Execute(devices, jobs, func(s *msg.Session) (any, error) {
		size, _, err := msg.CheckCoredump(s, 5*time.Second)
		if err != nil {
			return nil, err
		}
		if size == 0 {
			return []byte{}, nil
		}
		if delete {
			return nil, msg.DeleteCoredump(s, 5*time.Second)
		}
		data, err := msg.ReadCoredump(s, 0, size, 5*time.Second)
		if err != nil {
			return nil, err
		}
		return data, nil
	})

	// prepare output
	var firstErr error
	table := make(map[string][]byte)
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		table[baseTopics[i]] = result.Value.([]byte)
	}

	return table, firstErr
}
