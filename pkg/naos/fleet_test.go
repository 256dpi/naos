package naos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFleetFilterDevices1(t *testing.T) {
	f := NewFleet()
	f.Devices = make(map[string]*Device)
	f.Devices["foo"] = &Device{
		Name:      "foo",
		BaseTopic: "/foo",
	}
	f.Devices["bar"] = &Device{
		Name:      "bar",
		BaseTopic: "/bar",
	}

	devices := f.FilterDevices("foo")
	assert.Len(t, devices, 1)
	assert.Equal(t, devices[0].Name, "foo")
}

func TestFleetDeviceBaseTopics(t *testing.T) {
	f := NewFleet()
	f.Devices = make(map[string]*Device)
	f.Devices["foo"] = &Device{
		Name:      "foo",
		BaseTopic: "/foo",
	}
	f.Devices["bar"] = &Device{
		Name:      "bar",
		BaseTopic: "/bar",
	}
	f.Devices["baz"] = &Device{
		Name:      "baz",
		BaseTopic: "/baz",
	}

	devices := f.FilterDevices("f*")
	assert.Len(t, devices, 1)
	assert.Equal(t, devices[0].Name, "foo")
}
