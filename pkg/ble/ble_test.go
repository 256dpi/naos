package ble

import (
	"testing"
	"time"
)

func TestChannelCloseAllowsLateNotification(t *testing.T) {
	ch := &transport{
		reads: make(chan []byte, 1),
		done:  make(chan struct{}),
		close: func() {},
	}

	ch.Close()

	ch.mutex.Lock()
	ch.reads <- []byte("late")
	ch.mutex.Unlock()

	done := make(chan struct{})
	go func() {
		_, _ = ch.Read()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("read did not return after close")
	}
}
