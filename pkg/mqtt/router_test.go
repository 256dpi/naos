package mqtt

import (
	"testing"
	"time"

	"github.com/256dpi/gomqtt/packet"
	"github.com/stretchr/testify/assert"
)

func TestRouter(t *testing.T) {
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
