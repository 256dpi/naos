package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

// UpdateStatus represents the status of a firmware update.
type UpdateStatus struct {
	Progress float64
	Error    error
}

// Update will perform a firmware update on the provided devices. If a callback
// is provided it will be called with the current status of the update. The
// returned list is aligned with the provided list.
func Update(backend Backend, devices []*Device, firmware []byte, jobs int, callback func(*Device, UpdateStatus)) ([]UpdateStatus, error) {
	// check devices
	if len(devices) == 0 {
		return nil, errors.New("zero devices")
	}

	// resolve devices
	list, err := backend.Resolve(devices)
	if err != nil {
		return nil, err
	}

	// prepare statuses
	statuses := make([]UpdateStatus, len(list))

	// execute update
	results := msg.Execute(list, jobs, func(i int, s *msg.Session) (any, error) {
		// perform update
		err := msg.Update(s, firmware, func(progress int) {
			// set progress
			statuses[i].Progress = float64(progress) / float64(len(firmware))

			// call callback if provided
			if callback != nil {
				callback(devices[i], statuses[i])
			}
		}, 5*time.Second)
		if err != nil {
			// set error
			statuses[i].Error = err

			// call callback if provided
			if callback != nil {
				callback(devices[i], statuses[i])
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
