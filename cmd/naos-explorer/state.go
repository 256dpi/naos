package main

import (
	"sync"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

type state struct {
	devices map[string]*device
	order   []string
	logger  *logger
	mutex   sync.RWMutex
}

func newState() *state {
	return &state{
		logger: newLogger(50),
	}
}

func (s *state) register(dev msg.Device) *device {
	return s.registerWithMeta(dev, "", "", "")
}

func (s *state) registerWithMeta(dev msg.Device, deviceName, appName, appVersion string) *device {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.devices == nil {
		s.devices = make(map[string]*device)
	}

	key := dev.ID()

	if existing, ok := s.devices[key]; ok {
		existing.UpdateLastSeen(time.Now())
		return existing
	}

	ed := newDevice(dev, deviceName, appName, appVersion)
	s.devices[key] = ed
	s.order = append(s.order, key)

	s.log("Discovered: %s", ed.ID())

	return ed
}

func (s *state) snapshot() []*device {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	list := make([]*device, 0, len(s.order))
	for _, id := range s.order {
		if dev, ok := s.devices[id]; ok {
			list = append(list, dev)
		}
	}

	return list
}

func (s *state) log(format string, args ...any) {
	s.logger.Append(format, args...)
}
