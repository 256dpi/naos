package ble

import (
	"fmt"
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
	channel msg.Channel
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

func (d *device) Open() (msg.Channel, error) {
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

	// prepare channel
	ch := &channel{
		dev:  d,
		char: chars[0],
		close: func() {
			d.mutex.Lock()
			_ = device.Disconnect()
			d.channel = nil
			d.mutex.Unlock()
		},
	}

	// subscribe to characteristic
	err = char.EnableNotifications(func(data []byte) {
		ch.mutex.Lock()
		defer ch.mutex.Unlock()
		for sub := range ch.subs.Range {
			queue := sub.(msg.Queue)
			select {
			case queue <- data:
			default:
				// drop message if queue is full
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return ch, nil
}

type channel struct {
	dev   *device
	char  bluetooth.DeviceCharacteristic
	subs  sync.Map
	close func()
	mutex sync.Mutex
}

func (c *channel) Width() int {
	return 10
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

	// write to characteristic
	err := write(c.char, bytes)
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
