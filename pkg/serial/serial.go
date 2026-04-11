package serial

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
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
	port    io.ReadWriteCloser
	channel *msg.Channel
	mutex   sync.Mutex
}

func (d *device) ID() string {
	return fmt.Sprintf("serial/%s", filepath.Base(d.path))
}

func (d *device) Type() string {
	return "Serial"
}

func (d *device) Name() string {
	return filepath.Base(d.path)
}

func (d *device) Open() (*msg.Channel, error) {
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

	// prepare transport
	ch := &transport{
		dev:     d,
		port:    port,
		scanner: bufio.NewScanner(port),
		close: func() {
			d.mutex.Lock()
			_ = port.Close()
			d.channel = nil
			d.mutex.Unlock()
		},
	}

	// create channel
	d.channel = msg.NewChannel(ch, d, 1)

	return d.channel, nil
}

type transport struct {
	dev     *device
	port    io.Writer
	scanner *bufio.Scanner
	close   func()
	mutex   sync.Mutex
	once    sync.Once
}

func (t *transport) Read() ([]byte, error) {
	for t.scanner.Scan() {
		// get line
		line := t.scanner.Bytes()
		if len(line) < 6 || string(line[:5]) != "NAOS!" {
			continue
		}

		// strip prefix and decode
		data, err := base64.StdEncoding.AppendDecode(nil, line[5:])
		if err != nil {
			return nil, err
		}

		return data, nil
	}

	return nil, t.scanner.Err()
}

func (t *transport) Write(bytes []byte) error {
	// acquire mutex
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// encode message
	encoded := append(base64.StdEncoding.AppendEncode([]byte("\nNAOS!"), bytes), '\n')

	// write to port
	_, err := t.port.Write(encoded)
	if err != nil {
		return err
	}

	return nil
}

func (t *transport) Close() {
	t.once.Do(func() {
		t.mutex.Lock()
		t.close()
		t.mutex.Unlock()
	})
}
