package main

import (
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/256dpi/naos/pkg/msg"
)

func paramTypeString(t msg.ParamType) string {
	switch t {
	case msg.ParamTypeRaw:
		return "Raw"
	case msg.ParamTypeString:
		return "String"
	case msg.ParamTypeBool:
		return "Bool"
	case msg.ParamTypeLong:
		return "Long"
	case msg.ParamTypeDouble:
		return "Double"
	case msg.ParamTypeAction:
		return "Action"
	default:
		return "Unknown"
	}
}

func paramModeShort(m msg.ParamMode) string {
	var parts []string
	if m&msg.ParamModeVolatile != 0 {
		parts = append(parts, "V")
	}
	if m&msg.ParamModeSystem != 0 {
		parts = append(parts, "S")
	}
	if m&msg.ParamModeApplication != 0 {
		parts = append(parts, "A")
	}
	if m&msg.ParamModeLocked != 0 {
		parts = append(parts, "L")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "")
}

func formatParamValue(info msg.ParamInfo, update msg.ParamUpdate) string {
	// handle actions
	if info.Type == msg.ParamTypeAction {
		return "<Action>"
	}

	// handle numbers
	if info.Type == msg.ParamTypeLong || info.Type == msg.ParamTypeDouble {
		return string(update.Value)
	}

	// handle booleans
	if info.Type == msg.ParamTypeBool {
		if string(update.Value) == "1" {
			return "<True>"
		}
		return "<False>"
	}

	/* handle strings */

	// check length
	if len(update.Value) == 0 {
		return "<None>"
	}

	// print valid utf8 strings
	if info.Type == msg.ParamTypeString && utf8.Valid(update.Value) {
		return string(update.Value)
	}

	return "<Binary>"
}

func metricKindString(k msg.MetricKind) string {
	switch k {
	case msg.MetricKindCounter:
		return "Counter"
	case msg.MetricKindGauge:
		return "Gauge"
	default:
		return "Unknown"
	}
}

func metricTypeString(t msg.MetricType) string {
	switch t {
	case msg.MetricTypeLong:
		return "Long"
	case msg.MetricTypeFloat:
		return "Float"
	case msg.MetricTypeDouble:
		return "Double"
	default:
		return "Unknown"
	}
}

func layoutSummary(layout msg.MetricLayout) string {
	if len(layout.Keys) == 0 {
		return "-"
	}
	var b strings.Builder
	for i, key := range layout.Keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(key)
		if i < len(layout.Values) {
			b.WriteString("=")
			b.WriteString(strings.Join(layout.Values[i], "/"))
		}
	}
	return b.String()
}

func formatMetricValues(values []float64) string {
	if len(values) == 0 {
		return "-"
	}
	var parts []string
	for i, v := range values {
		if i >= 4 {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, strconv.FormatFloat(v, 'f', 3, 64))
	}
	return strings.Join(parts, ", ")
}

func humanDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "0s"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func previewFile(data []byte) string {
	if len(data) == 0 {
		return "(Empty File)"
	}
	if utf8.Valid(data) {
		return string(data)
	}
	return hex.Dump(data)
}

func pathJoin(base, name string) string {
	if base == "/" {
		return path.Join(base, name)
	}
	return path.Join(base, name)
}
