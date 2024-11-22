package msg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandom(t *testing.T) {
	assert.Len(t, random(16), 16)
}

func TestPacking(t *testing.T) {
	buf := pack("ohiqsob", byte(1), uint16(2), uint32(3), uint64(4), "hello", byte(0), []byte("world"))
	assert.Equal(t, []byte{
		1,
		2, 0,
		3, 0, 0, 0,
		4, 0, 0, 0, 0, 0, 0, 0,
		'h', 'e', 'l', 'l', 'o', 0,
		'w', 'o', 'r', 'l', 'd',
	}, buf)

	var o byte
	var h uint16
	var i uint32
	var q uint64
	var s string
	var n byte
	var b []byte
	unpack("ohiqsob", buf, &o, &h, &i, &q, &s, &n, &b)
	assert.Equal(t, byte(1), o)
	assert.Equal(t, uint16(2), h)
	assert.Equal(t, uint32(3), i)
	assert.Equal(t, uint64(4), q)
	assert.Equal(t, "hello", s)
	assert.Equal(t, byte(0), n)
	assert.Equal(t, []byte("world"), b)
}
