package fleet

import (
	"errors"

	"github.com/256dpi/naos/pkg/msg"
)

// DiscoverResult represents the result of a discovery operation.
type DiscoverResult struct {
	Params  map[string]string
	Metrics []string
}

// Discover will discover all available parameters and metrics for the
// specified devices. The returned list is aligned with the provided list.
func Discover(backend Backend, devices []*Device, jobs int) ([]DiscoverResult, error) {
	// check devices
	if len(devices) == 0 {
		return nil, errors.New("zero devices")
	}

	// resolve devices
	list, err := backend.Resolve(devices)
	if err != nil {
		return nil, err
	}

	// execute discover
	results := msg.Execute(list, jobs, func(_ int, s *msg.Session) (any, error) {
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
	output := make([]DiscoverResult, len(results))
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		output[i] = result.Value.(DiscoverResult)
	}

	return output, firstErr
}
