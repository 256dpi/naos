package naos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFleetFilterDevices1(t *testing.T) {
	f := NewFleet()
	f.Devices = make(map[string]*Device)
	f.Devices["foo"] = &Device{
		DeviceName: "foo",
		BaseTopic:  "/foo",
	}
	f.Devices["bar"] = &Device{
		DeviceName: "bar",
		BaseTopic:  "/bar",
	}

	devices := f.FilterDevices("foo")
	assert.Len(t, devices, 1)
	assert.Equal(t, devices[0].DeviceName, "foo")
}

func TestFleetDeviceBaseTopics(t *testing.T) {
	f := NewFleet()
	f.Devices = make(map[string]*Device)
	f.Devices["foo"] = &Device{
		DeviceName: "foo",
		BaseTopic:  "/foo",
	}
	f.Devices["bar"] = &Device{
		DeviceName: "bar",
		BaseTopic:  "/bar",
	}
	f.Devices["baz"] = &Device{
		DeviceName: "baz",
		BaseTopic:  "/baz",
	}

	devices := f.FilterDevices("f*")
	assert.Len(t, devices, 1)
	assert.Equal(t, devices[0].DeviceName, "foo")
}
