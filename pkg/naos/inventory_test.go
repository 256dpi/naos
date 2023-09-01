package naos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInventoryFilterDevices1(t *testing.T) {
	i := NewInventory()
	i.Devices = make(map[string]*Device)
	i.Devices["foo"] = &Device{
		Name:      "foo",
		BaseTopic: "/foo",
	}
	i.Devices["bar"] = &Device{
		Name:      "bar",
		BaseTopic: "/bar",
	}

	devices := i.FilterDevices("foo")
	assert.Len(t, devices, 1)
	assert.Equal(t, devices[0].Name, "foo")
}

func TestInventoryDeviceBaseTopics(t *testing.T) {
	i := NewInventory()
	i.Devices = make(map[string]*Device)
	i.Devices["foo"] = &Device{
		Name:      "foo",
		BaseTopic: "/foo",
	}
	i.Devices["bar"] = &Device{
		Name:      "bar",
		BaseTopic: "/bar",
	}
	i.Devices["baz"] = &Device{
		Name:      "baz",
		BaseTopic: "/baz",
	}

	devices := i.FilterDevices("f*")
	assert.Len(t, devices, 1)
	assert.Equal(t, devices[0].Name, "foo")
}
