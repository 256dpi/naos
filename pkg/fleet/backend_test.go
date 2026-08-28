package fleet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/naos/pkg/msg"
)

type fakeBackend struct {
	source string
	ann    []*Announcement
	seen   [][]*Device
}

func (b *fakeBackend) Collect(time.Duration) ([]*Announcement, error) {
	return b.ann, nil
}

func (b *fakeBackend) Resolve(devices []*Device) ([]msg.Device, error) {
	// record call
	b.seen = append(b.seen, devices)

	// create marker devices
	list := make([]msg.Device, 0, len(devices))
	for _, d := range devices {
		list = append(list, &missingDevice{name: b.source + "/" + d.DeviceName})
	}

	return list, nil
}

func (b *fakeBackend) Close() {}

func TestMultiBackendResolve(t *testing.T) {
	mqttBE := &fakeBackend{source: SourceMQTT}
	hubBE := &fakeBackend{source: SourceHub}

	backend := &multiBackend{
		backends: map[string]Backend{SourceMQTT: mqttBE, SourceHub: hubBE},
		fallback: SourceMQTT,
	}

	devices := []*Device{
		{Source: SourceHub, DeviceName: "a"},
		{Source: SourceMQTT, DeviceName: "b"},
		{DeviceName: "c"},                   // falls back to MQTT
		{Source: "serial", DeviceName: "d"}, // no backend
		{Source: SourceHub, DeviceName: "e"},
	}

	list, err := backend.Resolve(devices)
	assert.NoError(t, err)
	assert.Len(t, list, len(devices))

	// results must be aligned with the provided devices
	assert.Equal(t, "hub/a", list[0].Name())
	assert.Equal(t, "mqtt/b", list[1].Name())
	assert.Equal(t, "mqtt/c", list[2].Name())
	assert.Equal(t, "d", list[3].Name())
	assert.Equal(t, "hub/e", list[4].Name())

	// each backend must be called exactly once with its own subset
	assert.Len(t, mqttBE.seen, 1)
	assert.Len(t, hubBE.seen, 1)
	assert.Equal(t, []string{"b", "c"}, names(mqttBE.seen[0]))
	assert.Equal(t, []string{"a", "e"}, names(hubBE.seen[0]))

	// a device without a backend must fail when opened
	_, err = list[3].Open()
	assert.EqualError(t, err, `no "serial" backend configured`)
}

func TestMultiBackendCollect(t *testing.T) {
	backend := &multiBackend{
		backends: map[string]Backend{
			SourceMQTT: &fakeBackend{ann: []*Announcement{{Source: SourceMQTT, DeviceName: "a"}}},
			SourceHub:  &fakeBackend{ann: []*Announcement{{Source: SourceHub, DeviceName: "b"}}},
		},
		fallback: SourceMQTT,
	}

	ann, err := backend.Collect(0)
	assert.NoError(t, err)
	assert.Len(t, ann, 2)
}

func TestHubBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/list", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"devices": []map[string]any{{
				"uuid":        "aaa",
				"device_id":   "aaa",
				"device_name": "foo",
				"app_name":    "test",
				"app_version": "1.0.0",
				"attach_url":  "ws://example.com/attach/aaa",
			}, {
				"uuid":      "ccc",
				"device_id": "ccc",
			}},
		})
	}))
	defer server.Close()

	backend, err := NewHubBackend(server.URL, "secret")
	assert.NoError(t, err)
	defer backend.Close()

	// collect must fall back to the device ID for unnamed devices
	ann, err := backend.Collect(0)
	assert.NoError(t, err)
	assert.Equal(t, []*Announcement{
		{Source: SourceHub, DeviceID: "aaa", DeviceName: "foo", AppName: "test", AppVersion: "1.0.0"},
		{Source: SourceHub, DeviceID: "ccc", DeviceName: "ccc"},
	}, ann)

	// resolve must yield a connect device and a failing device
	list, err := backend.Resolve([]*Device{
		{Source: SourceHub, DeviceID: "aaa", DeviceName: "foo"},
		{Source: SourceHub, DeviceID: "bbb", DeviceName: "bar"},
	})
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "Connect", list[0].Type())
	assert.Equal(t, "Missing", list[1].Type())

	_, err = list[1].Open()
	assert.EqualError(t, err, `device "bar" not connected to hub`)
}

func TestFleetSourceFallback(t *testing.T) {
	for _, item := range []struct {
		fleet  string
		source string
	}{
		{`{"broker":"tcp://localhost:1883","devices":{"a":{"base_topic":"/a"}}}`, SourceMQTT},
		{`{"hub_url":"http://localhost:8080","devices":{"a":{"device_id":"x"}}}`, SourceHub},
		{`{"broker":"tcp://x","hub_url":"http://y","devices":{"a":{"base_topic":"/a"}}}`, SourceMQTT},
	} {
		path := filepath.Join(t.TempDir(), "fleet.json")
		assert.NoError(t, os.WriteFile(path, []byte(item.fleet), 0644))

		f, err := ReadFleet(path)
		assert.NoError(t, err)
		assert.Equal(t, item.source, f.Devices["a"].Source)
	}
}

func TestDeviceAddress(t *testing.T) {
	assert.Equal(t, "/a", (&Device{Source: SourceMQTT, BaseTopic: "/a", DeviceID: "x"}).Address())
	assert.Equal(t, "x", (&Device{Source: SourceHub, BaseTopic: "/a", DeviceID: "x"}).Address())
}

func names(devices []*Device) []string {
	var l []string
	for _, d := range devices {
		l = append(l, d.DeviceName)
	}
	return l
}
