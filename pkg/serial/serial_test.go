package serial

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransportReadParsesFrames(t *testing.T) {
	pr, pw := io.Pipe()

	ch := &transport{
		scanner: bufio.NewScanner(pr),
		close:   func() { pr.Close() },
	}

	payload := []byte{0x01, 0x02, 0x03}
	line := "NAOS!" + base64.StdEncoding.EncodeToString(payload) + "\n"

	go pw.Write([]byte("some debug log\n" + line))

	data, err := ch.Read()
	assert.NoError(t, err)
	assert.Equal(t, payload, data)
}

func TestTransportWriteFramesMessage(t *testing.T) {
	var buf bytes.Buffer

	ch := &transport{
		port:  &buf,
		close: func() {},
	}

	payload := []byte{0x01, 0x02, 0x03}
	err := ch.Write(payload)
	assert.NoError(t, err)

	want := "\nNAOS!" + base64.StdEncoding.EncodeToString(payload) + "\n"
	assert.Equal(t, want, buf.String())
}

func TestTransportReadReturnsOnClose(t *testing.T) {
	pr, pw := io.Pipe()

	ch := &transport{
		scanner: bufio.NewScanner(pr),
		close:   func() { pr.Close() },
	}

	done := make(chan struct{})
	go func() {
		_, err := ch.Read()
		assert.Error(t, err)
		close(done)
	}()

	ch.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		assert.Fail(t, "read did not return after close")
	}

	_ = pw
}
