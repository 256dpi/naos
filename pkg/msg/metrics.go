package msg

import (
	"encoding/binary"
	"errors"
	"math"
	"time"
)

const metricsEndpoint = 0x5

// MetricKind represents a metric kind.
type MetricKind uint8

// The available metric kinds.
const (
	MetricKindCounter MetricKind = iota
	MetricKindGauge
)

// MetricType represents a metric type.
type MetricType uint8

// The available metric types.
const (
	MetricTypeLong MetricType = iota
	MetricTypeFloat
	MetricTypeDouble
)

// MetricInfo describes a metric.
type MetricInfo struct {
	Ref  uint8
	Kind MetricKind
	Type MetricType
	Name string
	Size uint8
}

// MetricLayout describes a metric layout.
type MetricLayout struct {
	Keys   []string
	Values [][]string
}

// ListMetrics lists all metrics.
func ListMetrics(s *Session, timeout time.Duration) ([]MetricInfo, error) {
	// send command
	cmd := pack("o", uint8(0))
	err := s.Send(metricsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// prepare list
	var list []MetricInfo

	for {
		// receive reply
		reply, err := s.Receive(metricsEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			return list, nil
		} else if err != nil {
			return nil, err
		}

		// verify reply
		if len(reply) < 4 {
			return nil, errors.New("invalid reply")
		}

		// unpack reply
		args := unpack("oooos", reply)

		// append info
		list = append(list, MetricInfo{
			Ref:  args[0].(uint8),
			Kind: MetricKind(args[1].(uint8)),
			Type: MetricType(args[2].(uint8)),
			Size: args[3].(uint8),
			Name: args[4].(string),
		})
	}
}

// DescribeMetric describes a metric.
func DescribeMetric(s *Session, ref uint8, timeout time.Duration) (*MetricLayout, error) {
	// send command
	cmd := pack("oo", uint8(1), ref)
	err := s.Send(metricsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// prepare list
	var keys []string
	var values [][]string

	for {
		// receive reply
		reply, err := s.Receive(metricsEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			break
		} else if err != nil {
			return nil, err
		}

		// verify reply
		if len(reply) < 1 {
			return nil, errors.New("invalid reply")
		}

		// handle key
		if reply[0] == 0 {
			// verify reply
			if len(reply) < 3 {
				return nil, errors.New("invalid reply")
			}

			// parse reply
			args := unpack("os", reply[1:])
			// num := args[0].(uint8)
			key := args[1].(string)

			// add key
			keys = append(keys, key)
			values = append(values, []string{})

			continue
		}

		// handle value
		if reply[0] == 1 {
			// verify reply
			if len(reply) < 4 {
				return nil, errors.New("invalid reply")
			}

			// parse reply
			args := unpack("oos", reply[1:])
			numKey := args[0].(uint8)
			// numValue := args[1].(uint8)
			value := args[2].(string)

			// add value
			values[numKey] = append(values[numKey], value)

			continue
		}
	}

	return &MetricLayout{
		Keys:   keys,
		Values: values,
	}, nil
}

// ReadMetrics reads metrics.
func ReadMetrics(s *Session, ref uint8, timeout time.Duration) ([]byte, error) {
	// send command
	cmd := pack("oo", uint8(2), ref)
	err := s.Send(metricsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// receive reply
	reply, err := s.Receive(metricsEndpoint, false, timeout)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// ReadLongMetrics reads long metrics.
func ReadLongMetrics(s *Session, ref uint8, timeout time.Duration) ([]int32, error) {
	// read metrics
	data, err := ReadMetrics(s, ref, timeout)
	if err != nil {
		return nil, err
	}

	// parse metrics
	var metrics []int32
	for i := 0; i < len(data); i += 8 {
		metrics = append(metrics, int32(binary.LittleEndian.Uint32(data[i:])))
	}

	return metrics, nil
}

// ReadFloatMetrics reads float metrics.
func ReadFloatMetrics(s *Session, ref uint8, timeout time.Duration) ([]float32, error) {
	// read metrics
	data, err := ReadMetrics(s, ref, timeout)
	if err != nil {
		return nil, err
	}

	// parse metrics
	var metrics []float32
	for i := 0; i < len(data); i += 8 {
		metrics = append(metrics, math.Float32frombits(binary.LittleEndian.Uint32(data[i:])))
	}

	return metrics, nil
}

// ReadDoubleMetrics reads double metrics.
func ReadDoubleMetrics(s *Session, ref uint8, timeout time.Duration) ([]float64, error) {
	// read metrics
	data, err := ReadMetrics(s, ref, timeout)
	if err != nil {
		return nil, err
	}

	// parse metrics
	var metrics []float64
	for i := 0; i < len(data); i += 8 {
		metrics = append(metrics, math.Float64frombits(binary.LittleEndian.Uint64(data[i:])))
	}

	return metrics, nil
}
