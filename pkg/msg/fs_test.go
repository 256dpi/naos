package msg

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFSEndpoint(t *testing.T) {
	if testing.Short() {
		return
	}

	dev := NewHTTPDevice("10.0.1.7")

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch)
	assert.NoError(t, err)
	assert.NotNil(t, s)

	info, err := StatPath(s, "/lol.txt", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, &FSInfo{Size: 3}, info)

	infos, err := ListDir(s, "/", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []FSInfo{
		{Name: "LOL.TXT", Size: 3},
	}, infos)

	data, err := ReadFile(s, "/lol.txt", nil, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []byte("lol"), data)

	buf := random(16)
	sha := sha256.Sum256(buf)

	err = WriteFile(s, "/test.txt", buf, nil, time.Second)
	assert.NoError(t, err)

	data, err = ReadFile(s, "/test.txt", nil, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, buf, data)

	err = RenamePath(s, "/test.txt", "/test2.txt", time.Second)
	assert.NoError(t, err)

	hash, err := SHA256File(s, "/test2.txt", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, sha[:], hash)

	err = RemovePath(s, "/test2.txt", time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)

	ch.Close()
}
