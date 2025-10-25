package msg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandom(t *testing.T) {
	assert.Len(t, random(16), 16)
}

func TestPacking(t *testing.T) {
	buf := Pack("ohiqsob", byte(1), uint16(2), uint32(3), uint64(4), "hello", byte(0), []byte("world"))
	assert.Equal(t, []byte{
		1,
		2, 0,
		3, 0, 0, 0,
		4, 0, 0, 0, 0, 0, 0, 0,
		'h', 'e', 'l', 'l', 'o', 0,
		'w', 'o', 'r', 'l', 'd',
	}, buf)

	args := Unpack("ohiqsob", buf)
	assert.Equal(t, byte(1), args[0].(byte))
	assert.Equal(t, uint16(2), args[1].(uint16))
	assert.Equal(t, uint32(3), args[2].(uint32))
	assert.Equal(t, uint64(4), args[3].(uint64))
	assert.Equal(t, "hello", args[4].(string))
	assert.Equal(t, byte(0), args[5].(byte))
	assert.Equal(t, []byte("world"), args[6].([]byte))
}
