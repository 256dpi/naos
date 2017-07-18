package ble

// Configuration holds a complete device configuration.
type Configuration struct {
	WiFiSSID         string
	WiFiPassword     string
	MQTTHost         string
	MQTTPort         string
	MQTTClientID     string
	MQTTUsername     string
	MQTTPassword     string
	DeviceType       string
	DeviceName       string
	BaseTopic        string
	ConnectionStatus string
}

// ReadConfiguration will read a complete device configuration.
func ReadConfiguration(d *Device) (*Configuration, error) {
	// prepare configuration
	c := &Configuration{}

	// prepare job
	job := map[Property]*string{
		WiFiSSIDProperty:         &c.WiFiSSID,
		WiFiPasswordProperty:     &c.WiFiPassword,
		MQTTHostProperty:         &c.MQTTHost,
		MQTTPortProperty:         &c.MQTTPort,
		MQTTClientIDProperty:     &c.MQTTClientID,
		MQTTUsernameProperty:     &c.MQTTUsername,
		MQTTPasswordProperty:     &c.MQTTPassword,
		DeviceTypeProperty:       &c.DeviceType,
		DeviceNameProperty:       &c.DeviceName,
		BaseTopicProperty:        &c.BaseTopic,
		ConnectionStatusProperty: &c.ConnectionStatus,
	}

	// execute job
	for property, field := range job {
		// read property
		str, err := d.Read(property)
		if err != nil {
			return nil, err
		}

		// save value
		*field = str
	}

	return c, nil
}
