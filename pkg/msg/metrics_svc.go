package msg

import (
	"errors"
	"iter"
	"time"

	"github.com/samber/lo"
)

var ErrMetricNotFound = errors.New("metric not found")

type MetricsService struct {
	session *Session
	infos   []MetricInfo
	byName  map[string]MetricInfo
	byRef   map[uint8]MetricInfo
	layouts map[uint8]MetricLayout
}

func NewMetricsService(session *Session) *MetricsService {
	return &MetricsService{
		session: session,
		byName:  make(map[string]MetricInfo),
		byRef:   make(map[uint8]MetricInfo),
		layouts: make(map[uint8]MetricLayout),
	}
}

func (s *MetricsService) List() error {
	// list metrics
	infos, err := ListMetrics(s.session, 5*time.Second)
	if err != nil {
		return err
	}

	// store infos
	s.infos = infos
	for _, metric := range infos {
		s.byName[metric.Name] = metric
		s.byRef[metric.Ref] = metric

		// get layout
		if metric.Size > 1 {
			layout, err := DescribeMetric(s.session, metric.Ref, 5*time.Second)
			if err != nil {
				return err
			}
			s.layouts[metric.Ref] = *layout
		}
	}

	return nil
}

func (s *MetricsService) All() iter.Seq2[MetricInfo, MetricLayout] {
	return func(yield func(MetricInfo, MetricLayout) bool) {
		for _, info := range s.infos {
			layout, _ := s.layouts[info.Ref]
			if !yield(info, layout) {
				return
			}
		}
	}
}

func (s *MetricsService) Read(name string) ([]float64, error) {
	// get ref
	metric, ok := s.byName[name]
	if !ok {
		return nil, ErrMetricNotFound
	}

	// read metrics
	switch metric.Type {
	case MetricTypeLong:
		values, err := ReadLongMetrics(s.session, metric.Ref, 5*time.Second)
		if err != nil {
			return nil, err
		}
		return lo.Map(values, func(value int32, _ int) float64 {
			return float64(value)
		}), nil
	case MetricTypeFloat:
		values, err := ReadFloatMetrics(s.session, metric.Ref, 5*time.Second)
		if err != nil {
			return nil, err
		}
		return lo.Map(values, func(value float32, _ int) float64 {
			return float64(value)
		}), nil
	case MetricTypeDouble:
		values, err := ReadDoubleMetrics(s.session, metric.Ref, 5*time.Second)
		if err != nil {
			return nil, err
		}
		return values, nil
	}

	return nil, nil
}
