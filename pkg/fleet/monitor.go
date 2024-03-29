package fleet

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/256dpi/gomqtt/client"
	"github.com/256dpi/gomqtt/packet"
)

// A Heartbeat is emitted by Monitor.
type Heartbeat struct {
	ReceivedAt      time.Time
	BaseTopic       string
	DeviceName      string
	DeviceType      string
	FirmwareVersion string
	FreeHeapSize    int64
	UpTime          time.Duration
	StartPartition  string
	BatteryLevel    float64 // -1, 0 - 1
	SignalStrength  int64   // -50 - -100
	CPU0Usage       float64 // 0 - 1
	CPU1Usage       float64 // 0 - 1
}

// Monitor will connect to the specified MQTT broker and listen on the passed
// base topics for heartbeats and call the supplied callback until the specified
// quit channel is closed.
//
// Note: Not correctly formatted heartbeats are ignored.
func Monitor(url string, baseTopics []string, quit chan struct{}, timeout time.Duration, cb func(*Heartbeat)) error {
	// check base topics
	if len(baseTopics) == 0 {
		return errors.New("zero base topics")
	}

	// prepare channels
	errs := make(chan error)

	// create client
	cl := client.New()

	// set callback
	cl.Callback = func(msg *packet.Message, err error) error {
		// send errors
		if err != nil {
			errs <- err
			return nil
		}

		// get data from payload
		data := strings.Split(string(msg.Payload), ",")

		// check length
		if len(data) < 6 {
			return nil
		}

		// convert integers
		freeHeapSize, _ := strconv.ParseInt(data[3], 10, 64)
		upTime, _ := strconv.ParseInt(data[4], 10, 64)

		// create heartbeat
		hb := &Heartbeat{
			ReceivedAt:      time.Now(),
			DeviceType:      data[0],
			FirmwareVersion: data[1],
			DeviceName:      data[2],
			FreeHeapSize:    freeHeapSize,
			UpTime:          time.Duration(upTime) * time.Millisecond,
			StartPartition:  data[5],
			BatteryLevel:    -1,
		}

		// check battery level
		if len(data) >= 7 {
			hb.BatteryLevel, _ = strconv.ParseFloat(data[6], 64)
		}

		// check signal strength
		if len(data) >= 8 {
			hb.SignalStrength, _ = strconv.ParseInt(data[7], 10, 64)
		}

		// check cpu usage
		if len(data) >= 9 {
			hb.CPU0Usage, _ = strconv.ParseFloat(data[8], 64)
		}
		if len(data) >= 10 {
			hb.CPU1Usage, _ = strconv.ParseFloat(data[9], 64)
		}

		// set base topic
		for _, baseTopic := range baseTopics {
			if strings.HasPrefix(msg.Topic, baseTopic) {
				hb.BaseTopic = baseTopic
			}
		}

		// call callback
		cb(hb)

		return nil
	}

	// connect to the broker using the provided url
	cf, err := cl.Connect(client.NewConfig(url))
	if err != nil {
		return err
	}

	// wait for ack
	err = cf.Wait(timeout)
	if err != nil {
		return err
	}

	// make sure client gets closed
	defer cl.Close()

	// prepare subscriptions
	var subs []packet.Subscription

	// add subscriptions
	for _, baseTopic := range baseTopics {
		subs = append(subs, packet.Subscription{
			Topic: baseTopic + "/naos/heartbeat",
			QOS:   0,
		})
	}

	// subscribe to next chunk topic
	sf, err := cl.SubscribeMultiple(subs)
	if err != nil {
		return err
	}

	// wait for ack
	err = sf.Wait(timeout)
	if err != nil {
		return err
	}

	// wait for error or quit
	select {
	case err = <-errs:
		return err
	case <-quit:
		// move on
	}

	// disconnect client
	cl.Disconnect()

	return nil
}
