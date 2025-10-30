package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: Resolve dependency on real device.

func TestParamsEndpoint(t *testing.T) {
	if testing.Short() {
		return
	}

	dev := NewHTTPDevice("10.0.1.7")

	ch, err := dev.Open()
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	s, err := OpenSession(ch)
	assert.NoError(t, err)
	assert.NotNil(t, s)

	val, err := GetParam(s, "var_s", time.Second)
	assert.NoError(t, err)

	err = SetParam(s, "var_s", []byte("test"), time.Second)
	assert.NoError(t, err)

	list, err := ListParams(s, time.Second)
	assert.NoError(t, err)
	assert.NotEmpty(t, list)

	var num uint8
	for _, p := range list {
		if p.Name == "var_s" {
			num = p.Ref
			break
		}
	}

	val, err = ReadParam(s, num, time.Second)
	assert.NoError(t, err)
	assert.NotEmpty(t, val)

	err = WriteParam(s, num, []byte("test"), time.Second)
	assert.NoError(t, err)

	var mp []uint8
	for _, p := range list {
		mp = append(mp, p.Ref)
	}

	updates, err := CollectParams(s, mp, 1, time.Second)
	assert.NoError(t, err)
	assert.NotEmpty(t, updates)

	err = ClearParam(s, num, time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)

	ch.Close()
}
