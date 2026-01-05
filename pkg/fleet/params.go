package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// GetParams will receive the provided parameter for all specified base topics.
func GetParams(url, param string, baseTopics []string, jobs int) (map[string]string, error) {
	return modifyParams(url, param, "", false, baseTopics, jobs)
}

// SetParams will set the provided parameter on all specified base topics.
func SetParams(url, param, value string, baseTopics []string, jobs int) (map[string]string, error) {
	return modifyParams(url, param, value, true, baseTopics, jobs)
}

// UnsetParams will unset the provided parameter on all specified base topics.
func UnsetParams(url, param string, baseTopics []string, jobs int) error {
	_, err := modifyParams(url, param, "", true, baseTopics, jobs)
	return err
}

func modifyParams(url, param, value string, set bool, baseTopics []string, jobs int) (map[string]string, error) {
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

	// execute set/get
	results := msg.Execute(devices, jobs, func(s *msg.Session) (any, error) {
		if set {
			err := msg.SetParam(s, param, []byte(value), 5*time.Second)
			if err != nil {
				return nil, err
			}
		}
		value, err := msg.GetParam(s, param, 5*time.Second)
		if err != nil {
			return nil, err
		}
		return string(value), nil
	})

	// prepare output
	var firstErr error
	table := make(map[string]string)
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		table[baseTopics[i]] = result.Value.(string)
	}

	return table, firstErr
}
