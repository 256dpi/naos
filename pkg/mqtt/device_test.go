package mqtt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannelCloseAllowsLateCallback(t *testing.T) {
	ch := &transport{
		reads: make(chan []byte, 1),
		done:  make(chan struct{}),
	}

	close(ch.done)

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
		assert.Fail(t, "read did not return after close")
	}
}
