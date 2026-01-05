package fleet

import (
	"errors"
	"strconv"
	"time"

	"github.com/256dpi/gomqtt/packet"
	"github.com/samber/lo"

	"github.com/256dpi/naos/pkg/mqtt"
	"github.com/256dpi/naos/pkg/msg"
)

// A Heartbeat is emitted by Monitor.
type Heartbeat struct {
	ReceivedAt     time.Time
	BaseTopic      string
	DeviceName     string
	AppName        string
	AppVersion     string
	AppPartition   string
	UpTime         time.Duration
	BatteryLevel   float64 // -1, 0 - 1
	FreeMemory     []float64
	CPUUsage       []float64 // 0 - 1
	SignalStrength int64     // -50 - -100
}

// Monitor will connect to the specified MQTT broker and listen on the passed
// base topics for heartbeats and call the supplied callback until the specified
// quit channel is closed.
//
// Note: Not correctly formatted heartbeats are ignored.
func Monitor(url string, baseTopics []string, stop chan struct{}, cb func(*Heartbeat)) error {
	// check base topics
	if len(baseTopics) == 0 {
		return errors.New("zero base topics")
	}

	// create router
	router, err := mqtt.Connect(url, "naos-fleet", packet.QOSAtMostOnce)
	if err != nil {
		return err
	}
	defer router.Close()

	// create devices
	devices := make([]msg.Device, 0, len(baseTopics))
	for _, baseTopic := range baseTopics {
		devices = append(devices, mqtt.NewDevice(router, baseTopic))
	}

	// TODO: Handle device resets.

	// execute discover
	results := msg.Execute(devices, len(devices), func(s *msg.Session) (any, error) {
		// get index and base topic
		index := lo.IndexOf(devices, s.Channel().Device())
		baseTopic := baseTopics[index]

		// prepare services
		ps := msg.NewParamsService(s)
		ms := msg.NewMetricsService(s)

		// list params
		err = ps.List()
		if err != nil {
			return nil, err
		}

		// list metrics
		err = ms.List()
		if err != nil {
			return nil, err
		}

		// collect static params
		err = ps.Collect("base-topic", "device-name", "app-name", "app-version", "app-partition")
		if err != nil {
			return nil, err
		}

		// read static params
		deviceName, _ := ps.Read("device-name", false)
		appName, _ := ps.Read("app-name", false)
		appVersion, _ := ps.Read("app-version", false)
		appPartition, _ := ps.Read("app-partition", false)

		for {
			select {
			case <-stop:
				return nil, nil
			default:
				// collect dynamic params
				var dynamicParams = []string{"uptime"}
				if ps.Has("battery") {
					dynamicParams = append(dynamicParams, "battery")
				}
				err = ps.Collect(dynamicParams...)
				if err != nil {
					return nil, err
				}

				// read dynamic params
				upTimeStr, _ := ps.Read("uptime", false)
				batteryLevelStr, _ := ps.Read("battery", false)

				// convert dynamic params
				uptime, _ := strconv.ParseInt(string(upTimeStr), 10, 64)
				batteryLevel, _ := strconv.ParseFloat(string(batteryLevelStr), 64)

				// read metrics
				freeMemory, err := ms.Read("free-memory")
				if err != nil {
					return nil, err
				}
				cpuUsage, err := ms.Read("cpu-usage")
				if err != nil {
					return nil, err
				}
				signalStrength, err := ms.Read("wifi-rssi")
				if err != nil {
					return nil, err
				}

				// create heartbeat
				hb := &Heartbeat{
					ReceivedAt:     time.Now(),
					BaseTopic:      baseTopic,
					DeviceName:     string(deviceName),
					AppName:        string(appName),
					AppVersion:     string(appVersion),
					AppPartition:   string(appPartition),
					UpTime:         time.Duration(uptime) * time.Millisecond,
					BatteryLevel:   batteryLevel,
					FreeMemory:     freeMemory,
					CPUUsage:       cpuUsage,
					SignalStrength: int64(signalStrength[0]),
				}

				// call callback
				cb(hb)

				// wait before next collection
				select {
				case <-stop:
					return nil, nil
				case <-time.After(5 * time.Second):
				}
			}
		}
	})
	for _, result := range results {
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}
