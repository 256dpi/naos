package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: Resolve dependency on real device.

func TestHTTPChannel(t *testing.T) {
	dev := NewHTTPDevice("10.0.1.7")

	ch, err := dev.Open()
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	s, err := OpenSession(ch)
	assert.NoError(t, err)
	assert.NotNil(t, s)

	for i := 0; i < 10; i++ {
		err = s.Ping(time.Second)
		assert.NoError(t, err)
	}

	err = s.End(time.Second)
	assert.NoError(t, err)

	ch.Close()
}
