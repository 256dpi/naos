package serial

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.bug.st/serial"

	"github.com/256dpi/naos/pkg/msg"
)

// BestDevice returns the first best available serial device.
func BestDevice() (msg.Device, error) {
	// get port list
	ports, err := ListPorts()
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no serial ports found")
	}

	// find best port
	path := ports[0]
	for _, port := range ports {
		if strings.Contains(port, "usbmodem") {
			path = port
			break
		}
	}

	// create device
	dev := &device{
		path: path,
	}

	return dev, nil
}

// NewDevice creates a new serial device for the given path.
func NewDevice(path string) (msg.Device, error) {
	// get port list
	ports, err := ListPorts()
	if err != nil {
		return nil, err
	}

	// check port
	if !slices.Contains(ports, path) {
		return nil, fmt.Errorf("serial port %q not found", path)
	}

	// create device
	dev := &device{
		path: path,
	}

	return dev, nil
}

type device struct {
	path    string
	port    serial.Port
	channel msg.Channel
	mutex   sync.Mutex
}

func (d *device) ID() string {
	return fmt.Sprintf("serial/%s", filepath.Base(d.path))
}

func (d *device) Open() (msg.Channel, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check channel
	if d.channel != nil {
		return nil, fmt.Errorf("channel already open")
	}

	// open device
	port, err := serial.Open(d.path, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		return nil, err
	}

	// prepare channel
	ch := &channel{
		dev:  d,
		port: port,
		close: func() {
			d.mutex.Lock()
			_ = port.Close()
			d.channel = nil
			d.mutex.Unlock()
		},
	}

	// read from port
	go func() {
		defer ch.Close()
		var scanner = bufio.NewScanner(port)
		for scanner.Scan() {
			// get line
			line := scanner.Bytes()
			if len(line) < 6 || string(line[:5]) != "NAOS!" {
				continue
			}

			// strip prefix and decode
			data, _ := base64.StdEncoding.AppendDecode(nil, line[5:])

			// call notification handler
			ch.mutex.Lock()
			for sub := range ch.subs.Range {
				queue := sub.(msg.Queue)
				select {
				case queue <- data:
				default:
					// drop message if queue is full
				}
			}
			ch.mutex.Unlock()
		}
	}()

	return ch, nil
}

type channel struct {
	dev   *device
	port  serial.Port
	subs  sync.Map
	close func()
	mutex sync.Mutex
}

func (c *channel) Width() int {
	return 1
}

func (c *channel) Device() msg.Device {
	return c.dev
}

func (c *channel) Subscribe(queue msg.Queue) {
	// add subscription
	c.subs.Store(queue, struct{}{})
}

func (c *channel) Unsubscribe(queue msg.Queue) {
	// remove subscription
	c.subs.Delete(queue)
}

func (c *channel) Write(bytes []byte) error {
	// acquire mutex
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// encode message
	encoded := append(base64.StdEncoding.AppendEncode([]byte("\nNAOS!"), bytes), '\n')

	// write to port
	_, err := c.port.Write(encoded)
	if err != nil {
		return err
	}

	return nil
}

func (c *channel) Close() {
	// acquire mutex
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// close subscriptions
	for sub := range c.subs.Range {
		close(sub.(msg.Queue))
	}

	// clear subscriptions
	c.subs = sync.Map{}

	// close device
	c.close()
}
