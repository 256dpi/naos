package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

// Debug will request coredump debug information from the specified devices. The
// returned list is aligned with the provided list.
func Debug(backend Backend, devices []*Device, delete bool, jobs int) ([][]byte, error) {
	// check devices
	if len(devices) == 0 {
		return nil, errors.New("zero devices")
	}

	// resolve devices
	list, err := backend.Resolve(devices)
	if err != nil {
		return nil, err
	}

	// execute
	results := msg.Execute(list, jobs, func(_ int, s *msg.Session) (any, error) {
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
	}, nil)

	// prepare output
	var firstErr error
	coredumps := make([][]byte, len(results))
	for i, result := range results {
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		coredumps[i] = result.Value.([]byte)
	}

	return coredumps, firstErr
}
