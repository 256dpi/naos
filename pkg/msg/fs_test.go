package msg

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatPath(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: fsEndpoint, Data: Pack("os", uint8(0), "/test.txt")}),
		send(Message{Endpoint: fsEndpoint, Data: Pack("ooi", uint8(1), uint8(0), uint32(42))}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	info, err := StatPath(s, "/test.txt", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, &FSInfo{Name: "test.txt", IsDir: false, Size: 42}, info)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestListDir(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: fsEndpoint, Data: Pack("os", uint8(1), "/")}),
		send(Message{Endpoint: fsEndpoint, Data: Pack("oois", uint8(1), uint8(0), uint32(42), "file.txt")}),
		send(Message{Endpoint: fsEndpoint, Data: Pack("oois", uint8(1), uint8(1), uint32(0), "subdir")}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	infos, err := ListDir(s, "/", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []FSInfo{
		{Name: "file.txt", IsDir: false, Size: 42},
		{Name: "subdir", IsDir: true, Size: 0},
	}, infos)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadFileRange(t *testing.T) {
	fileData := []byte("0123456789")

	dev := newTestDevice(t, 42, []testMessage{
		// open
		receive(Message{Endpoint: fsEndpoint, Data: Pack("oos", uint8(2), uint8(0), "/test.txt")}),
		ack(),
		// read
		receive(Message{Endpoint: fsEndpoint, Data: Pack("oii", uint8(3), uint32(0), uint32(10))}),
		// chunk
		send(Message{Endpoint: fsEndpoint, Data: Pack("oib", uint8(2), uint32(0), fileData)}),
		ack(),
		// close
		receive(Message{Endpoint: fsEndpoint, Data: Pack("o", uint8(5))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	data, err := ReadFileRange(s, "/test.txt", 0, 10, nil, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, fileData, data)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestWriteFile(t *testing.T) {
	testData := bytes.Repeat([]byte{0xAA}, 50)

	dev := newTestDevice(t, 42, []testMessage{
		// create
		receive(Message{Endpoint: fsEndpoint, Data: Pack("oos", uint8(2), uint8((1<<0)|(1<<2)), "/test.txt")}),
		ack(),
		// GetMTU
		receive(Message{Endpoint: SystemEndpoint, Data: Pack("o", uint8(2))}),
		send(Message{Endpoint: SystemEndpoint, Data: Pack("h", uint16(30))}),
		// chunk 0: 24 bytes, acked (mode=2, sequential)
		receive(Message{Endpoint: fsEndpoint, Data: Pack("ooib", uint8(4), uint8(2), uint32(0), testData[0:24])}),
		ack(),
		// chunk 1: 24 bytes, not acked (mode=3, silent & sequential)
		receive(Message{Endpoint: fsEndpoint, Data: Pack("ooib", uint8(4), uint8(3), uint32(24), testData[24:48])}),
		// chunk 2: 2 bytes, acked as last chunk (mode=2, sequential)
		receive(Message{Endpoint: fsEndpoint, Data: Pack("ooib", uint8(4), uint8(2), uint32(48), testData[48:50])}),
		ack(),
		// close
		receive(Message{Endpoint: fsEndpoint, Data: Pack("o", uint8(5))}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = WriteFile(s, "/test.txt", testData, nil, time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestRenamePath(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: fsEndpoint, Data: Pack("osos", uint8(6), "/old.txt", uint8(0), "/new.txt")}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = RenamePath(s, "/old.txt", "/new.txt", time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestRemovePath(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: fsEndpoint, Data: Pack("os", uint8(7), "/test.txt")}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = RemovePath(s, "/test.txt", time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestSHA256File(t *testing.T) {
	hash := bytes.Repeat([]byte{0xAB}, 32)

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: fsEndpoint, Data: Pack("os", uint8(8), "/test.txt")}),
		send(Message{Endpoint: fsEndpoint, Data: append([]byte{3}, hash...)}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	result, err := SHA256File(s, "/test.txt", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, hash, result)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestMakePath(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: fsEndpoint, Data: Pack("os", uint8(9), "/newdir")}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	err = MakePath(s, "/newdir", time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
