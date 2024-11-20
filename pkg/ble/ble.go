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

	return Discover(ctx, func(device msg.Device) {
		// get channel
		ch, err := device.Channel()
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
		utils.Log(out, fmt.Sprintf("Configured: %s", device.Addr()))
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

	// prepare map
	devices := map[string]bool{}

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

		// check map
		if devices[result.Address.String()] {
			return
		}

		// mark device
		devices[result.Address.String()] = true

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

func (d *device) Addr() string {
	return fmt.Sprintf("ble/%s", d.addr.String())
}

func (d *device) Channel() (msg.Channel, error) {
	// acquire mutex
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// check channel
	if d.channel != nil {
		return d.channel, nil
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
		char: chars[0],
		subs: map[msg.Queue]struct{}{},
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
		for sub := range ch.subs {
			select {
			case sub <- data:
			default:
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return ch, nil
}

type channel struct {
	char  bluetooth.DeviceCharacteristic
	subs  map[msg.Queue]struct{}
	close func()
	mutex sync.Mutex
}

func (c *channel) Name() string {
	return "ble"
}

func (c *channel) Subscribe(ch msg.Queue) {
	// acquire mutex
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// add subscription
	c.subs[ch] = struct{}{}
}

func (c *channel) Unsubscribe(ch msg.Queue) {
	// acquire mutex
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// remove subscription
	delete(c.subs, ch)
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
	for sub := range c.subs {
		close(sub)
	}

	// clear subscriptions
	c.subs = map[msg.Queue]struct{}{}

	// close device
	c.close()
}
