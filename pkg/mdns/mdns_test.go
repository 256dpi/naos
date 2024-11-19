package mdns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: Resolve dependency on real device.

func TestDiscover(t *testing.T) {
	locations, err := Discover(time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []Location{
		{
			Hostname: "test1234.local.",
			Address:  "10.0.1.7",
		},
	}, locations)
}
