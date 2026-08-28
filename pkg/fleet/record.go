package fleet

import (
	"errors"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

// A LogMessage receive from a device.
type LogMessage struct {
	Time    time.Time
	Device  *Device
	Content string
}

// Record will enable log recording mode on all devices and yield the received
// log messages until the provided channel has been closed. Devices that fail
// are reported via the error callback as soon as they fail, potentially from
// multiple goroutines.
func Record(backend Backend, devices []*Device, stop chan struct{}, cb func(*LogMessage), onError func(*Device, error)) error {
	// check devices
	if len(devices) == 0 {
		return errors.New("zero devices")
	}

	// resolve devices
	list, err := backend.Resolve(devices)
	if err != nil {
		return err
	}

	// execute log streaming
	results := msg.Execute(list, len(list), func(i int, s *msg.Session) (any, error) {
		return nil, msg.StreamLog(s, stop, func(content string) {
			// call callback
			cb(&LogMessage{
				Time:    time.Now(),
				Device:  devices[i],
				Content: content,
			})
		})
	}, func(i int, result msg.Result) {
		// report device error if requested
		if result.Error != nil && onError != nil {
			onError(devices[i], result.Error)
		}
	})
	for _, result := range results {
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}
