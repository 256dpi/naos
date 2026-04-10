package ble

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/samber/lo"
	"tinygo.org/x/bluetooth"

	"github.com/256dpi/naos/pkg/msg"
)

var adapter = bluetooth.DefaultAdapter
var serviceUUID = lo.Must(bluetooth.ParseUUID("632FBA1B-4861-4E4F-8103-FFEE9D5033B5"))
var msgUUID = lo.Must(bluetooth.ParseUUID("0360744B-A61B-00AD-C945-37F3634130F3"))

// The Description of a BLE device.
type Description struct {
	Address bluetooth.Address
	RSSI    int
	Name    string
}

// Discover scans for BLE devices and calls the provided callback for each
// discovered device.
func Discover(stop chan struct{}, cb func(device Description)) error {
	// enable BLE adapter
	err := adapter.Enable()
	if err != nil && !strings.Contains(err.Error(), "already calling Enable function") {
		return err
	}

	// handle cancel
	go func() {
		<-stop
		_ = adapter.StopScan()
	}()

	// start scanning
	err = adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		// check service
		if !result.HasServiceUUID(serviceUUID) {
			return
		}

		// yield device
		go cb(Description{
			Address: result.Address,
			RSSI:    int(result.RSSI),
			Name:    result.LocalName(),
		})
	})

	return nil
}

type device struct {
	addr    bluetooth.Address
	channel *msg.Channel
	mutex   sync.Mutex
}

// NewDevice creates a new BLE device with the given address.
func NewDevice(addr bluetooth.Address) msg.Device {
	return &device{
		addr: addr,
	}
}

func (d *device) ID() string {
	return fmt.Sprintf("ble/%s", d.addr.String())
}

func (d *device) Open() (*msg.Channel, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check channel
	if d.channel != nil {
		return nil, fmt.Errorf("channel already open")
	}

	// connect to device
	device, err := adapter.Connect(d.addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}

	// discover services
	svcs, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil {
		return nil, err
	}

	// check services
	if len(svcs) != 1 {
		return nil, fmt.Errorf("unexpected number of services: %d", len(svcs))
	}

	// discover characteristics
	chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{
		msgUUID,
	})
	if err != nil {
		return nil, err
	}

	// check characteristics
	if len(chars) != 1 {
		return nil, fmt.Errorf("unexpected number of characteristics: %d", len(chars))
	}

	// get characteristic
	char := chars[0]

	// prepare transport
	ch := &transport{
		dev:   d,
		char:  chars[0],
		reads: make(chan []byte, 64),
		done:  make(chan struct{}),
		close: func() {
			d.mutex.Lock()
			_ = device.Disconnect()
			d.channel = nil
			d.mutex.Unlock()
		},
	}

	// subscribe to characteristic
	err = char.EnableNotifications(func(data []byte) {
		select {
		case ch.reads <- append([]byte(nil), data...):
		default:
			// TODO: Alert overflow?
		}
	})
	if err != nil {
		return nil, err
	}

	// create channel
	d.channel = msg.NewChannel(ch, d, 10)

	return d.channel, nil
}

type transport struct {
	dev   *device
	char  bluetooth.DeviceCharacteristic
	reads chan []byte
	done  chan struct{}
	close func()
	mutex sync.Mutex
	once  sync.Once
}

func (t *transport) Read() ([]byte, error) {
	// read async message
	select {
	case data := <-t.reads:
		return data, nil
	case <-t.done:
		return nil, io.EOF
	}
}

func (t *transport) Write(bytes []byte) error {
	// acquire mutex
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// write to characteristic
	err := write(t.char, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (t *transport) Close() {
	// close transport
	t.once.Do(func() {
		close(t.done)
		t.mutex.Lock()
		t.close()
		t.mutex.Unlock()
	})
}
