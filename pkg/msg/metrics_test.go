package msg

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestListMetrics(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: metricsEndpoint, Data: Pack("o", uint8(0))}),
		send(Message{Endpoint: metricsEndpoint, Data: Pack("oooos", uint8(1), uint8(MetricKindCounter), uint8(MetricTypeLong), uint8(1), "cpu")}),
		send(Message{Endpoint: metricsEndpoint, Data: Pack("oooos", uint8(2), uint8(MetricKindGauge), uint8(MetricTypeFloat), uint8(2), "temp")}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	list, err := ListMetrics(s, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []MetricInfo{
		{Ref: 1, Kind: MetricKindCounter, Type: MetricTypeLong, Size: 1, Name: "cpu"},
		{Ref: 2, Kind: MetricKindGauge, Type: MetricTypeFloat, Size: 2, Name: "temp"},
	}, list)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestDescribeMetric(t *testing.T) {
	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: metricsEndpoint, Data: Pack("oo", uint8(1), uint8(2))}),
		send(Message{Endpoint: metricsEndpoint, Data: Pack("oos", uint8(0), uint8(0), "dimension")}),
		send(Message{Endpoint: metricsEndpoint, Data: Pack("ooos", uint8(1), uint8(0), uint8(0), "x")}),
		send(Message{Endpoint: metricsEndpoint, Data: Pack("ooos", uint8(1), uint8(0), uint8(1), "y")}),
		ack(),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	layout, err := DescribeMetric(s, 2, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, &MetricLayout{
		Keys:   []string{"dimension"},
		Values: [][]string{{"x", "y"}},
	}, layout)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadLongMetrics(t *testing.T) {
	v := int32(-200)
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], 100)
	binary.LittleEndian.PutUint32(data[4:], uint32(v))

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: metricsEndpoint, Data: Pack("oo", uint8(2), uint8(1))}),
		send(Message{Endpoint: metricsEndpoint, Data: data}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	metrics, err := ReadLongMetrics(s, 1, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []int32{100, -200}, metrics)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadFloatMetrics(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], math.Float32bits(1.5))
	binary.LittleEndian.PutUint32(data[4:], math.Float32bits(2.5))

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: metricsEndpoint, Data: Pack("oo", uint8(2), uint8(1))}),
		send(Message{Endpoint: metricsEndpoint, Data: data}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	metrics, err := ReadFloatMetrics(s, 1, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []float32{1.5, 2.5}, metrics)

	err = s.End(time.Second)
	assert.NoError(t, err)
}

func TestReadDoubleMetrics(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data[0:], math.Float64bits(1.5))
	binary.LittleEndian.PutUint64(data[8:], math.Float64bits(2.5))

	dev := newTestDevice(t, 42, []testMessage{
		receive(Message{Endpoint: metricsEndpoint, Data: Pack("oo", uint8(2), uint8(1))}),
		send(Message{Endpoint: metricsEndpoint, Data: data}),
	})

	ch, err := dev.Open()
	assert.NoError(t, err)

	s, err := OpenSession(ch, time.Second)
	assert.NoError(t, err)

	metrics, err := ReadDoubleMetrics(s, 1, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, []float64{1.5, 2.5}, metrics)

	err = s.End(time.Second)
	assert.NoError(t, err)
}
