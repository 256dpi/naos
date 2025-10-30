package msg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: Resolve dependency on real device.

func TestMetricsEndpoint(t *testing.T) {
	if testing.Short() {
		return
	}
	
	dev := NewHTTPDevice("10.0.1.11")

	ch, err := dev.Open()
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	s, err := OpenSession(ch)
	assert.NoError(t, err)
	assert.NotNil(t, s)

	metrics, err := ListMetrics(s, time.Second)
	assert.NoError(t, err)
	assert.NotEmpty(t, metrics)

	for _, m := range metrics {
		layout, err := DescribeMetric(s, m.Ref, time.Second)
		assert.NoError(t, err)
		if m.Size > 1 {
			assert.NotEmpty(t, layout)
		} else {
			assert.Empty(t, layout)
		}

		switch m.Type {
		case MetricTypeLong:
			data, err := ReadLongMetrics(s, m.Ref, time.Second)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		case MetricTypeFloat:
			data, err := ReadFloatMetrics(s, m.Ref, time.Second)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		case MetricTypeDouble:
			data, err := ReadDoubleMetrics(s, m.Ref, time.Second)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		}
	}

	err = s.End(time.Second)
	assert.NoError(t, err)

	ch.Close()
}
