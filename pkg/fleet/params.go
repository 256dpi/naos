package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

// GetParams will receive the provided parameter for all specified devices. The
// returned list is aligned with the provided list.
func GetParams(backend Backend, param string, devices []*Device, jobs int) ([]string, error) {
	return modifyParams(backend, param, "", false, devices, jobs)
}

// SetParams will set the provided parameter on all specified devices. The
// returned list is aligned with the provided list.
func SetParams(backend Backend, param, value string, devices []*Device, jobs int) ([]string, error) {
	return modifyParams(backend, param, value, true, devices, jobs)
}

// UnsetParams will unset the provided parameter on all specified devices.
func UnsetParams(backend Backend, param string, devices []*Device, jobs int) error {
	_, err := modifyParams(backend, param, "", true, devices, jobs)
	return err
}

func modifyParams(backend Backend, param, value string, set bool, devices []*Device, jobs int) ([]string, error) {
	// check devices
	if len(devices) == 0 {
		return nil, errors.New("zero devices")
	}

	// resolve devices
	list, err := backend.Resolve(devices)
	if err != nil {
		return nil, err
	}

	// execute set/get
	results := msg.Execute(list, jobs, func(_ int, s *msg.Session) (any, error) {
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
	values := make([]string, len(results))
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		values[i] = result.Value.(string)
	}

	return values, firstErr
}
