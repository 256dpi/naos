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
	mutex     sync.Mutex
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
		// acquire mutex
		r.mutex.Lock()
		defer r.mutex.Unlock()

		// handle errors
		if err != nil {
			for _, cbs := range r.callbacks {
				for _, cb := range cbs {
					cb.fn(nil, err)
				}
			}
			return err
		}

		// call callbacks
		for _, cb := range r.callbacks[msg.Topic] {
			cb.fn(msg, nil)
		}

		return nil
	}

	return r, nil
}

// Subscribe subscribes to the given topic and registers the provided callback
// function. It returns a handle that can be used to unsubscribe later.
func (r *Router) Subscribe(topic string, fn func(*packet.Message, error)) (uint64, error) {
	// acquire mutex
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// generate id
	r.counter++
	id := r.counter

	// subscribe topic if first subscriber
	if len(r.callbacks[topic]) == 0 {
		sf, err := r.client.Subscribe(topic, r.qos)
		if err != nil {
			return 0, err
		}
		err = sf.Wait(5 * time.Second)
		if err != nil {
			return 0, err
		}
	}

	// register callback
	r.callbacks[topic] = append(r.callbacks[topic], callback{
		id: id,
		fn: fn,
	})

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
	// acquire mutex
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// remove callback
	callbacks := r.callbacks[topic]
	for i, cb := range callbacks {
		if cb.id == id {
			r.callbacks[topic] = append(callbacks[:i], callbacks[i+1:]...)
			break
		}
	}

	// unsubscribe topic if no more subscribers
	if len(r.callbacks[topic]) == 0 {
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
	// acquire mutex
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// cancel callbacks
	for _, cbs := range r.callbacks {
		for _, cb := range cbs {
			go cb.fn(nil, fmt.Errorf("router closed"))
		}
	}

	// disconnect client
	err := r.client.Disconnect()
	if err != nil {
		return err
	}

	return nil
}
