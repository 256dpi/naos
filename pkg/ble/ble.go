package ble

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"tinygo.org/x/bluetooth"

	"github.com/256dpi/naos/pkg/msg"
	"github.com/256dpi/naos/pkg/utils"
)

var adapter = bluetooth.DefaultAdapter
var serviceUUID = lo.Must(bluetooth.ParseUUID("632FBA1B-4861-4E4F-8103-FFEE9D5033B5"))
var msgUUID = lo.Must(bluetooth.ParseUUID("0360744B-A61B-00AD-C945-37F3634130F3"))

// Config configures all reachable BLE device with the given parameters.
func Config(params map[string]string, timeout time.Duration, out io.Writer) error {
	// prepare context
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// log info
	utils.Log(out, "Scanning for devices... (press Ctrl+C to stop)")

	// prepare registry
	registry := map[string]bool{}

	return Discover(ctx, func(device msg.Device) {
		// check registry
		if registry[device.ID()] {
			return
		}

		// mark device
		registry[device.ID()] = true

		// get channel
		ch, err := device.Open()
		if err != nil {
			utils.Log(out, fmt.Sprintf("Error: %s", err))
			return
		}
		defer ch.Close()

		// open session
		s, err := msg.OpenSession(ch)
		if err != nil {
			utils.Log(out, fmt.Sprintf("Error: %s", err))
			return
		}
		defer s.End(time.Second)

		// write parameters
		for param, value := range params {
			err = msg.SetParam(s, param, []byte(value), time.Second)
			if err != nil {
				utils.Log(out, fmt.Sprintf("Error: %s", err))
				return
			}
		}

		// log success
		utils.Log(out, fmt.Sprintf("Configured: %s", device.ID()))
	})
}

// Discover scans for BLE devices and calls the provided callback for each
// discovered device.
func Discover(ctx context.Context, cb func(device msg.Device)) error {
	// enable BLE adapter
	err := adapter.Enable()
	if err != nil && !strings.Contains(err.Error(), "already calling Enable function") {
		return err
	}

	// handle cancel
	go func() {
		<-ctx.Done()
		_ = adapter.StopScan()
	}()

	// start scanning
	err = adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		// check service
		if !result.HasServiceUUID(serviceUUID) {
			return
		}

		// yield device
		go cb(&device{addr: result.Address})
	})

	return nil
}

type device struct {
	addr    bluetooth.Address
	channel msg.Channel
	mutex   sync.Mutex
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
