package ble

import "github.com/currantlabs/ble"

// A Property represents a readable and/or writable property.
type Property string

// All available properties.
const (
	WiFiSSIDProperty         Property = "802DD327-CA04-4C90-BE86-A3568275A510"
	WiFiPasswordProperty              = "B3883261-F360-4CB7-9791-C3498FB2C151"
	MQTTHostProperty                  = "193FFFF2-4542-4EBC-BE1F-4A355D40AC57"
	MQTTPortProperty                  = "CB8A764C-546C-4787-9A6F-427382D49755"
	MQTTClientIDProperty              = "08C4E543-65CC-4BF6-BBA8-C8A5BF916C3B"
	MQTTUsernameProperty              = "ABB4A59A-85E9-449C-80F5-72D806A00257"
	MQTTPasswordProperty              = "C5ECB1B1-0658-4FC3-8052-215521D41925"
	DeviceTypeProperty                = "0CEA3120-196E-4308-8883-8EA8757B9191"
	DeviceNameProperty                = "25427850-28F3-40EA-926A-AB3CEB6D0B56"
	BaseTopicProperty                 = "EAB7E3A8-9FF8-4938-8EA2-D09B8C63CAB4"
	ConnectionStatusProperty          = "59997CE0-3A50-433C-A200-D9F6D312EB7C"
	systemCommandProperty             = "37CF1864-5A8E-450F-A277-2981BD76D0AB"
)

// Readable returns whether the property is readable.
func (p Property) Readable() bool {
	switch p {
	case WiFiSSIDProperty, WiFiPasswordProperty, MQTTHostProperty, MQTTPortProperty, MQTTClientIDProperty, MQTTUsernameProperty, MQTTPasswordProperty, DeviceTypeProperty, DeviceNameProperty, BaseTopicProperty, ConnectionStatusProperty:
		return true
	default:
		return false
	}
}

// Writable returns whether the property is writable.
func (p Property) Writable() bool {
	switch p {
	case WiFiSSIDProperty, WiFiPasswordProperty, MQTTHostProperty, MQTTPortProperty, MQTTClientIDProperty, MQTTUsernameProperty, MQTTPasswordProperty, DeviceNameProperty, BaseTopicProperty, systemCommandProperty:
		return true
	case DeviceTypeProperty, ConnectionStatusProperty:
		return false
	default:
		return false
	}
}

func (p Property) uuid() (ble.UUID, error) {
	return ble.Parse(string(p))
}
