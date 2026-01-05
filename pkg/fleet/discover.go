package fleet

import (
	"errors"

	"github.com/256dpi/gomqtt/packet"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// DiscoverResult represents the result of a discovery operation.
type DiscoverResult struct {
	Params  map[string]string
	Metrics []string
}

// Discover will connect to the specified MQTT broker and discover all available
// parameters and metrics for the specified base topics.
func Discover(url string, baseTopics []string, jobs int) (map[string]DiscoverResult, error) {
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
		ps := msg.NewParamsService(s)
		ms := msg.NewMetricsService(s)
		err := ps.List()
		if err != nil {
			return nil, err
		}
		err = ps.Collect()
		if err != nil {
			return nil, err
		}
		err = ms.List()
		if err != nil {
			return nil, err
		}
		result := DiscoverResult{
			Params: make(map[string]string),
		}
		for info, update := range ps.All() {
			result.Params[info.Name] = string(update.Value)
		}
		for info := range ms.All() {
			result.Metrics = append(result.Metrics, info.Name)
		}
		return result, nil
	})

	// prepare output
	var firstErr error
	table := make(map[string]DiscoverResult)
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		table[baseTopics[i]] = result.Value.(DiscoverResult)
	}

	return table, firstErr
}
