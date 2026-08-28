package fleet

import (
	"errors"
	"time"

	"github.com/samber/lo"

	"github.com/256dpi/naos/pkg/msg"
)

// UpdateStatus represents the status of a firmware update.
type UpdateStatus struct {
	Progress float64
	Error    error
}

// Update will perform a firmware update on the provided devices. If a callback
// is provided it will be called with the current status of the update.
func Update(backend Backend, baseTopics []string, firmware []byte, jobs int, callback func(string, UpdateStatus)) ([]UpdateStatus, error) {
	// check base topics
	if len(baseTopics) == 0 {
		return nil, errors.New("zero base topics")
	}

	// get devices
	devices, err := backend.Devices(baseTopics)
	if err != nil {
		return nil, err
	}

	// map topics to devices
	deviceTopics := make(map[string]msg.Device)
	for i, baseTopic := range baseTopics {
		deviceTopics[baseTopic] = devices[i]
	}

	// prepare statuses
	statuses := make([]UpdateStatus, len(devices))

	// execute log streaming
	results := msg.Execute(devices, jobs, func(s *msg.Session) (any, error) {
		// get index and base topic
		index := lo.IndexOf(devices, s.Channel().Device())
		baseTopic := baseTopics[index]

		// perform update
		err := msg.Update(s, firmware, func(progress int) {
			// set progress
			statuses[index].Progress = float64(progress) / float64(len(firmware))

			// call callback if provided
			if callback != nil {
				callback(baseTopic, statuses[index])
			}
		}, 5*time.Second)
		if err != nil {
			// set error
			statuses[index].Error = err

			// call callback if provided
			if callback != nil {
				callback(baseTopic, statuses[index])
			}
		}

		return nil, err
	})

	// get first error
	var firstErr error
	for _, result := range results {
		if result.Error != nil {
			firstErr = result.Error
			break
		}
	}

	return statuses, firstErr
}
