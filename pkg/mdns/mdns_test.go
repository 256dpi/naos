package mdns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: Resolve dependency on real device.

func TestCollect(t *testing.T) {
	if testing.Short() {
		return
	}

	locations, err := Collect(time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []Descriptor{
		{
			Hostname: "test1234.local.",
			Address:  "10.0.1.7",
		},
	}, locations)
}
