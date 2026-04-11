package connect

import (
	"fmt"
	"io"

	"github.com/256dpi/naos/pkg/msg"
)

// Bridge will forward and return messages from the provided transport to the
// given channel.
func Bridge(t *Transport, c *msg.Channel) error {
	// create queue
	queue := make(msg.Queue, 128)

	// subscribe queue
	c.Subscribe(queue)

	// ensure cleanup
	defer func() {
		c.Unsubscribe(queue)
		t.Close()
	}()

	// catch errors
	errs := make(chan error, 1)

	// run reader
	go func() {
		for {
			data, err := t.Read()
			if err != nil {
				errs <- err
				return
			}
			m, ok := msg.Parse(data)
			if !ok {
				errs <- fmt.Errorf("invalid message")
				return
			}
			err = c.Write(queue, m)
			if err != nil {
				errs <- err
				return
			}
		}
	}()

	// run writer
	for {
		select {
		case err := <-errs:
			return err
		case <-c.Done():
			return io.EOF
		case m := <-queue:
			err := t.Write(m.Build())
			if err != nil {
				return err
			}
		}
	}
}
