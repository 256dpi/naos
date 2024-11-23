package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestManagedDevice(t *testing.T) {
	dev := NewHTTPDevice("10.0.1.7")

	device := NewManagedDevice(dev)
	assert.False(t, device.Active())

	err := device.Activate()
	assert.NoError(t, err)
	assert.True(t, device.Active())

	err = device.Activate()
	assert.NoError(t, err)
	assert.True(t, device.Active())

	_ = device.UseSession(func(session *Session) error {
		ok, err := session.Query(ParamsEndpoint, time.Second)
		assert.NoError(t, err)
		assert.True(t, ok)
		return nil
	})

	session, err := device.NewSession()
	assert.NoError(t, err)

	ok, err := session.Query(ParamsEndpoint, time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	err = session.End(time.Second)
	assert.NoError(t, err)

	device.Deactivate()
	assert.False(t, device.Active())

	device.Deactivate()
	assert.False(t, device.Active())

	device.Stop()
}
