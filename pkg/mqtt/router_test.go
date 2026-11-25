package mqtt

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/256dpi/gomqtt/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := Connect("mqtt://localhost:1883", "test-router", 0)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	defer r.client.Close()

	var sub1 []any
	s1, err := r.Subscribe("test/topic", func(msg *packet.Message, err error) {
		sub1 = append(sub1, string(msg.Payload))
	})
	assert.NoError(t, err)

	var sub2 []any
	s2, err := r.Subscribe("test/topic", func(msg *packet.Message, err error) {
		sub2 = append(sub2, string(msg.Payload))
	})
	assert.NoError(t, err)

	_, err = r.client.Publish("test/topic", []byte("hello"), r.qos, false)
	assert.NoError(t, err)

	_, err = r.client.Publish("test/foo", []byte("hello"), r.qos, false)
	assert.NoError(t, err)

	// wait a bit for messages to be processed
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, []any{"hello"}, sub1)
	assert.Equal(t, []any{"hello"}, sub2)

	// unsubscribe first subscriber
	err = r.Unsubscribe("test/topic", s1)
	assert.NoError(t, err)

	_, err = r.client.Publish("test/topic", []byte("world"), r.qos, false)
	assert.NoError(t, err)

	// wait a bit for messages to be processed
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, []any{"hello"}, sub1)          // should remain unchanged
	assert.Equal(t, []any{"hello", "world"}, sub2) // should have new message

	// unsubscribe second subscriber
	err = r.Unsubscribe("test/topic", s2)
	assert.NoError(t, err)

	_, err = r.client.Publish("test/topic", []byte("!"), r.qos, false)
	assert.NoError(t, err)

	// wait a bit for messages to be processed
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, []any{"hello"}, sub1)          // should remain unchanged
	assert.Equal(t, []any{"hello", "world"}, sub2) // should remain unchanged
}

func TestRouterSubscribeWhileReceiving(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := Connect("mqtt://localhost:1883", "test-router", 0)
	require.NoError(t, err)

	defer r.client.Close()

	// receive a steady stream of messages
	var received atomic.Int64
	_, err = r.Subscribe("test/stream", func(msg *packet.Message, err error) {
		received.Add(1)
	})
	require.NoError(t, err)

	pub, err := Connect("mqtt://localhost:1883", "test-router-pub", 0)
	require.NoError(t, err)

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			default:
				_ = pub.Publish("test/stream", []byte("x"))
			}
		}
	}()
	defer func() {
		close(done)
		<-stopped
		_ = pub.client.Close()
	}()

	require.Eventually(t, func() bool {
		return received.Load() > 100
	}, time.Second, 10*time.Millisecond)

	// subscribing and unsubscribing must not stall while messages arrive, as
	// parallel sessions do when one opens while another is answered
	for i := 0; i < 10; i++ {
		topic := fmt.Sprintf("test/other/%d", i)
		start := time.Now()
		id, err := r.Subscribe(topic, func(*packet.Message, error) {})
		require.NoError(t, err)
		err = r.Unsubscribe(topic, id)
		require.NoError(t, err)
		require.Less(t, time.Since(start), time.Second)
	}
}
