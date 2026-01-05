package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// Discover will connect to the specified MQTT broker and publish the 'discover'
// command to receive a list of available parameters.
func Discover(url string, baseTopics []string, jobs int) (map[string][]string, error) {
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

	// execute discover
	results := msg.Execute(devices, jobs, func(s *msg.Session) (any, error) {
		infos, err := msg.ListParams(s, 5*time.Second)
		if err != nil {
			return nil, err
		}
		var params []string
		for _, info := range infos {
			params = append(params, info.Name)
		}
		return params, nil
	})

	// prepare output
	var firstErr error
	table := make(map[string][]string)
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		table[baseTopics[i]] = result.Value.([]string)
	}

	return table, firstErr
}

// GetParams will connect to the specified MQTT broker and publish the 'get'
// command to receive the provided parameter for all specified base topics.
func GetParams(url, param string, baseTopics []string, jobs int) (map[string]string, error) {
	return commonGetSet(url, param, "", false, baseTopics, jobs)
}

// SetParams will connect to the specified MQTT broker and publish the 'set'
// command to receive the provided updated parameter for all specified base topics.
func SetParams(url, param, value string, baseTopics []string, jobs int) (map[string]string, error) {
	return commonGetSet(url, param, value, true, baseTopics, jobs)
}

// UnsetParams will connect to the specified MQTT broker and publish the 'unset'
// command to unset the provided parameter for all specified base topics.
func UnsetParams(url, param string, baseTopics []string, jobs int) error {
	_, err := commonGetSet(url, param, "", true, baseTopics, jobs)
	return err
}

func commonGetSet(url, param, value string, set bool, baseTopics []string, jobs int) (map[string]string, error) {
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
