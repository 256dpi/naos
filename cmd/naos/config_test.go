package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/naos/pkg/msg"
)

func TestPlanWrite(t *testing.T) {
	host := msg.ParamInfo{Ref: 0, Type: msg.ParamTypeString, Mode: msg.ParamModeSystem, Name: "mqtt-host"}
	port := msg.ParamInfo{Ref: 1, Type: msg.ParamTypeLong, Mode: msg.ParamModeSystem, Name: "mqtt-port"}
	topic := msg.ParamInfo{Ref: 2, Type: msg.ParamTypeString, Mode: msg.ParamModeSystem, Name: "base-topic"}
	uptime := msg.ParamInfo{Ref: 3, Type: msg.ParamTypeLong, Mode: msg.ParamModeVolatile, Name: "uptime"}
	reboot := msg.ParamInfo{Ref: 4, Type: msg.ParamTypeAction, Mode: msg.ParamModeSystem, Name: "reboot"}
	version := msg.ParamInfo{Ref: 5, Type: msg.ParamTypeString, Mode: msg.ParamModeSystem | msg.ParamModeLocked, Name: "app-version"}

	plan := planWrite([]msg.ParamInfo{host, port, topic, uptime, reboot, version}, map[uint8][]byte{
		0: []byte("broker.local"),
		1: []byte("1883"),
		2: []byte(""),
		3: []byte("42"),
		5: []byte("v1.0.0"),
	}, map[string]string{
		"mqtt-host":   "192.168.1.215",
		"mqtt-port":   "1883",
		"base-topic":  "airlab",
		"uptime":      "0",
		"reboot":      "",
		"app-version": "v2.0.0",
		"missing":     "x",
	})

	assert.Equal(t, writePlan{
		changes: []paramChange{
			{info: topic, old: "", value: "airlab"},
			{info: host, old: "broker.local", value: "192.168.1.215"},
		},
		unchanged: 1,
		skipped:   []string{"app-version", "reboot", "uptime"},
		unknown:   []string{"missing"},
	}, plan)
}

func TestPlanWriteUnchanged(t *testing.T) {
	host := msg.ParamInfo{Ref: 0, Type: msg.ParamTypeString, Mode: msg.ParamModeSystem, Name: "mqtt-host"}

	plan := planWrite([]msg.ParamInfo{host}, map[uint8][]byte{
		0: []byte("broker.local"),
	}, map[string]string{
		"mqtt-host": "broker.local",
	})

	assert.Equal(t, writePlan{unchanged: 1}, plan)
}

func TestPrintable(t *testing.T) {
	assert.Equal(t, "", printable(nil))
	assert.Equal(t, "broker.local", printable([]byte("broker.local")))
	assert.Equal(t, "µg/m³", printable([]byte("µg/m³")))
	assert.Equal(t, "0x00ff", printable([]byte{0x00, 0xff}))
	assert.Equal(t, "0x610a", printable([]byte("a\n")))
}

func TestModeName(t *testing.T) {
	assert.Equal(t, "", modeName(0))
	assert.Equal(t, "system,volatile,locked", modeName(msg.ParamModeSystem|msg.ParamModeVolatile|msg.ParamModeLocked))
	assert.Equal(t, "application", modeName(msg.ParamModeApplication))
}
