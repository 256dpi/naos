package serial

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"testing"
	"time"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("unexpected data: %v", data)
	}
}

func TestTransportWriteFramesMessage(t *testing.T) {
	var buf bytes.Buffer

	ch := &transport{
		port:  &buf,
		close: func() {},
	}

	payload := []byte{0x01, 0x02, 0x03}
	err := ch.Write(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "\nNAOS!" + base64.StdEncoding.EncodeToString(payload) + "\n"
	if buf.String() != want {
		t.Fatalf("unexpected frame: %q want %q", buf.String(), want)
	}
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
		if err == nil {
			t.Error("expected error after close")
		}
		close(done)
	}()

	ch.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("read did not return after close")
	}

	_ = pw
}
