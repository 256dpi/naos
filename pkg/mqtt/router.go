package mqtt

import (
	"fmt"
	"sync"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// TODO: Use service and persistent sessions?

type callback struct {
	id uint64
	fn func(*packet.Message, error)
}

// Router provides a multiplexed MQTT client with topic based callbacks.
type Router struct {
	client    *client.Client
	qos       packet.QOS
	counter   uint64
	callbacks map[string][]callback
	// The callback map is also read for every incoming message, on the client's
	// only receive goroutine, so its mutex must never be held across a broker
	// round trip: the acknowledgement would queue behind a message waiting for
	// the mutex. Subscription changes are serialized by their own mutex.
	mutex   sync.Mutex
	changes sync.Mutex
}

// Connect creates a new Router connected to the given MQTT broker URL
// using the provided client ID and QOS level.
func Connect(url, cid string, qos packet.QOS) (*Router, error) {
	// create client
	c := client.New()

	// connect to the broker using the provided url
	cf, err := c.Connect(client.NewConfigWithClientID(url, cid))
	if err != nil {
		return nil, err
	}
	err = cf.Wait(5 * time.Second)
	if err != nil {
		return nil, err
	}

	// create router
	r := &Router{
		client:    c,
		qos:       qos,
		callbacks: make(map[string][]callback),
	}

	// set handler
	c.Callback = func(msg *packet.Message, err error) error {
		// cancel all callbacks on error
		if err != nil {
			for _, cb := range r.snapshotCallbacks(nil) {
				cb.fn(nil, err)
			}
			return err
		}

		// otherwise, forward message
		for _, cb := range r.snapshotCallbacks(msg) {
			cb.fn(msg, nil)
		}

		return nil
	}

	return r, nil
}

// Subscribe subscribes to the given topic and registers the provided callback
// function. It returns a handle that can be used to unsubscribe later.
func (r *Router) Subscribe(topic string, fn func(*packet.Message, error)) (uint64, error) {
	// serialize subscription changes
	r.changes.Lock()
	defer r.changes.Unlock()

	// register callback before subscribing, so that no message delivered right
	// after the acknowledgement is missed
	r.mutex.Lock()
	r.counter++
	id := r.counter
	first := len(r.callbacks[topic]) == 0
	r.callbacks[topic] = append(r.callbacks[topic], callback{
		id: id,
		fn: fn,
	})
	r.mutex.Unlock()

	// subscribe topic if first subscriber
	if first {
		sf, err := r.client.Subscribe(topic, r.qos)
		if err == nil {
			err = sf.Wait(5 * time.Second)
		}
		if err != nil {
			r.remove(topic, id)
			return 0, err
		}
	}

	return id, nil
}

// Publish publishes the given payload to the specified topic.
func (r *Router) Publish(topic string, payload []byte) error {
	// publish message
	sf, err := r.client.Publish(topic, payload, r.qos, false)
	if err != nil {
		return err
	}
	err = sf.Wait(5 * time.Second)
	if err != nil {
		return err
	}

	return nil
}

// Unsubscribe removes the callback with the given handle from the topic and
// unsubscribes from the topic if there are no more subscribers.
func (r *Router) Unsubscribe(topic string, id uint64) error {
	// serialize subscription changes
	r.changes.Lock()
	defer r.changes.Unlock()

	// remove callback
	last := r.remove(topic, id)

	// unsubscribe topic if no more subscribers
	if last {
		sf, err := r.client.Unsubscribe(topic)
		if err != nil {
			return err
		}
		err = sf.Wait(5 * time.Second)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close closes the router and disconnects the underlying client.
func (r *Router) Close() error {
	// collect all callbacks
	r.mutex.Lock()
	callbacks := make([]callback, 0, len(r.callbacks))
	for _, cbs := range r.callbacks {
		callbacks = append(callbacks, cbs...)
	}
	r.callbacks = make(map[string][]callback)
	r.mutex.Unlock()

	// cancel callbacks
	for _, cb := range callbacks {
		go cb.fn(nil, fmt.Errorf("router closed"))
	}

	// disconnect client
	err := r.client.Disconnect()
	if err != nil {
		return err
	}

	return nil
}

// remove drops the callback with the given handle from the topic and reports
// whether the topic has no subscribers left.
func (r *Router) remove(topic string, id uint64) bool {
	// acquire mutex
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// remove callback
	callbacks := r.callbacks[topic]
	for i, cb := range callbacks {
		if cb.id == id {
			callbacks = append(callbacks[:i:i], callbacks[i+1:]...)
			break
		}
	}

	// drop topic if empty
	if len(callbacks) == 0 {
		delete(r.callbacks, topic)
		return true
	}
	r.callbacks[topic] = callbacks

	return false
}

func (r *Router) snapshotCallbacks(msg *packet.Message) []callback {
	// acquire mutex
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// collect matching callbacks
	if msg != nil {
		return append([]callback(nil), r.callbacks[msg.Topic]...)
	}

	// collect all callbacks
	callbacks := make([]callback, 0, len(r.callbacks))
	for _, cbs := range r.callbacks {
		callbacks = append(callbacks, cbs...)
	}

	return callbacks
}
