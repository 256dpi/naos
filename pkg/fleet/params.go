package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

// GetParams will receive the provided parameter for all specified base topics.
func GetParams(backend Backend, param string, baseTopics []string, jobs int) (map[string]string, error) {
	return modifyParams(backend, param, "", false, baseTopics, jobs)
}

// SetParams will set the provided parameter on all specified base topics.
func SetParams(backend Backend, param, value string, baseTopics []string, jobs int) (map[string]string, error) {
	return modifyParams(backend, param, value, true, baseTopics, jobs)
}

// UnsetParams will unset the provided parameter on all specified base topics.
func UnsetParams(backend Backend, param string, baseTopics []string, jobs int) error {
	_, err := modifyParams(backend, param, "", true, baseTopics, jobs)
	return err
}

func modifyParams(backend Backend, param, value string, set bool, baseTopics []string, jobs int) (map[string]string, error) {
	// check base topics
	if len(baseTopics) == 0 {
		return nil, errors.New("zero base topics")
	}

	// get devices
	devices, err := backend.Devices(baseTopics)
	if err != nil {
		return nil, err
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
