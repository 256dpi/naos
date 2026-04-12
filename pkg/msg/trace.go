package msg

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const traceEndpoint = 0x8

// TraceEventType represents a trace event type.
type TraceEventType uint8

// The available trace event types.
const (
	TraceTaskSwitch TraceEventType = 1
	TraceInstant    TraceEventType = 2
	TraceBegin      TraceEventType = 3
	TraceEnd        TraceEventType = 4
	TraceValue      TraceEventType = 5
)

// TraceTask describes a task in the trace task table.
type TraceTask struct {
	ID   uint8
	Name string
}

// TraceLabel describes a label in the trace label table.
type TraceLabel struct {
	ID   uint8
	Text string
}

// TraceEvent represents a single trace event.
type TraceEvent struct {
	Timestamp uint32         // microseconds since trace start
	Type      TraceEventType // event type
	Core      uint8          // CPU core (SWITCH only)
	Task      uint8          // task ID (SWITCH only)
	Cat       uint8          // category label ID (INSTANT/BEGIN only)
	Name      uint8          // name label ID (INSTANT/BEGIN/VALUE only)
	Arg       uint16         // user argument (INSTANT/BEGIN only)
	Span      uint8          // span instance ID (BEGIN/END only)
	Value     int32          // counter/gauge value (VALUE only)
}

// TraceStatus represents the trace buffer status.
type TraceStatus struct {
	Active  bool
	BufSize uint32
	BufUsed uint32
	Dropped uint32
}

// TraceData holds the result of a trace read operation.
type TraceData struct {
	Tasks  []TraceTask
	Labels []TraceLabel
	Events []TraceEvent
}

// StartTrace begins trace recording.
func StartTrace(s *Session, timeout time.Duration) error {
	return s.Send(traceEndpoint, []byte{0}, timeout)
}

// StopTrace stops trace recording.
func StopTrace(s *Session, timeout time.Duration) error {
	return s.Send(traceEndpoint, []byte{1}, timeout)
}

// ReadTrace reads buffered trace events and any new task/label mappings.
func ReadTrace(s *Session, timeout time.Duration) (*TraceData, error) {
	// send READ command
	err := s.Send(traceEndpoint, []byte{2}, 0)
	if err != nil {
		return nil, err
	}

	// prepare data
	data := &TraceData{}

	for {
		// read chunk
		chunk, err := s.Receive(traceEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			break
		}
		if err != nil {
			return nil, err
		}

		// parse variable-length records from chunk
		err = parseTraceRecords(chunk, data)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func parseTraceRecords(buf []byte, data *TraceData) error {
	pos := 0
	for pos < len(buf) {
		remaining := len(buf) - pos
		switch buf[pos] {
		case 1: // SWITCH: TYPE(1) TS(4) CORE(1) ID(1) = 7
			if remaining < 7 {
				return fmt.Errorf("truncated SWITCH record")
			}
			data.Events = append(data.Events, TraceEvent{
				Timestamp: binary.LittleEndian.Uint32(buf[pos+1:]),
				Type:      TraceTaskSwitch,
				Core:      buf[pos+5],
				Task:      buf[pos+6],
			})
			pos += 7

		case 2: // EVENT: TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) = 9
			if remaining < 9 {
				return fmt.Errorf("truncated EVENT record")
			}
			data.Events = append(data.Events, TraceEvent{
				Timestamp: binary.LittleEndian.Uint32(buf[pos+1:]),
				Type:      TraceInstant,
				Cat:       buf[pos+5],
				Name:      buf[pos+6],
				Arg:       binary.LittleEndian.Uint16(buf[pos+7:]),
			})
			pos += 9

		case 3: // BEGIN: TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) ID(1) = 10
			if remaining < 10 {
				return fmt.Errorf("truncated BEGIN record")
			}
			data.Events = append(data.Events, TraceEvent{
				Timestamp: binary.LittleEndian.Uint32(buf[pos+1:]),
				Type:      TraceBegin,
				Cat:       buf[pos+5],
				Name:      buf[pos+6],
				Arg:       binary.LittleEndian.Uint16(buf[pos+7:]),
				Span:      buf[pos+9],
			})
			pos += 10

		case 4: // END: TYPE(1) TS(4) ID(1) = 6
			if remaining < 6 {
				return fmt.Errorf("truncated END record")
			}
			data.Events = append(data.Events, TraceEvent{
				Timestamp: binary.LittleEndian.Uint32(buf[pos+1:]),
				Type:      TraceEnd,
				Span:      buf[pos+5],
			})
			pos += 6

		case 5: // VALUE: TYPE(1) TS(4) CAT(1) NAME(1) VAL(4) = 11
			if remaining < 11 {
				return fmt.Errorf("truncated VALUE record")
			}
			data.Events = append(data.Events, TraceEvent{
				Timestamp: binary.LittleEndian.Uint32(buf[pos+1:]),
				Type:      TraceValue,
				Cat:       buf[pos+5],
				Name:      buf[pos+6],
				Value:     int32(binary.LittleEndian.Uint32(buf[pos+7:])),
			})
			pos += 11

		case 6: // LABEL: TYPE(1) ID(1) TEXT(*) NUL(1)
			if remaining < 3 {
				return fmt.Errorf("truncated LABEL record")
			}
			nulIdx := bytes.IndexByte(buf[pos+2:], 0)
			if nulIdx < 0 {
				return fmt.Errorf("unterminated LABEL record")
			}
			data.Labels = append(data.Labels, TraceLabel{
				ID:   buf[pos+1],
				Text: string(buf[pos+2 : pos+2+nulIdx]),
			})
			pos += 2 + nulIdx + 1

		case 7: // TASK: TYPE(1) ID(1) NAME(*) NUL(1)
			if remaining < 3 {
				return fmt.Errorf("truncated TASK record")
			}
			nulIdx := bytes.IndexByte(buf[pos+2:], 0)
			if nulIdx < 0 {
				return fmt.Errorf("unterminated TASK record")
			}
			data.Tasks = append(data.Tasks, TraceTask{
				ID:   buf[pos+1],
				Name: string(buf[pos+2 : pos+2+nulIdx]),
			})
			pos += 2 + nulIdx + 1

		default:
			return fmt.Errorf("unknown record type: %d at offset %d", buf[pos], pos)
		}
	}
	return nil
}

// GetTraceStatus returns the current trace buffer status.
func GetTraceStatus(s *Session, timeout time.Duration) (*TraceStatus, error) {
	// send STATUS command
	err := s.Send(traceEndpoint, []byte{3}, 0)
	if err != nil {
		return nil, err
	}

	// receive reply
	reply, err := s.Receive(traceEndpoint, false, timeout)
	if err != nil {
		return nil, err
	}

	// verify reply: ACTIVE(1) | BUF_SIZE(4) | BUF_USED(4) | DROPPED(4)
	if len(reply) != 13 {
		return nil, fmt.Errorf("invalid status reply length: %d", len(reply))
	}

	return &TraceStatus{
		Active:  reply[0] != 0,
		BufSize: binary.LittleEndian.Uint32(reply[1:]),
		BufUsed: binary.LittleEndian.Uint32(reply[5:]),
		Dropped: binary.LittleEndian.Uint32(reply[9:]),
	}, nil
}

type perfettoEvent struct {
	Name string         `json:"name"`
	Cat  string         `json:"cat,omitempty"`
	Ph   string         `json:"ph"`
	Ts   uint32         `json:"ts"`
	Dur  uint32         `json:"dur,omitempty"`
	Pid  uint32         `json:"pid"`
	Tid  uint32         `json:"tid,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

// GeneratePerfetto converts trace data into the Chrome Trace Event Format
// (compatible with Perfetto). Task names and labels are provided as maps
// built from accumulated reads during the trace recording.
func GeneratePerfetto(tasks map[uint8]string, labels map[uint8]string, events []TraceEvent) ([]byte, error) {
	var out []perfettoEvent

	// make tid unique per core so Perfetto doesn't merge threads
	// across processes (it keys thread_name by tid alone)
	makeTid := func(core uint8, task uint8) uint32 {
		return uint32(core)*1000 + uint32(task)
	}

	// collect which core+task pairs appear in the data
	type coreTid struct {
		core uint8
		tid  uint8
	}
	seen := map[coreTid]bool{}
	for _, e := range events {
		if e.Type == TraceTaskSwitch {
			seen[coreTid{e.Core, e.Task}] = true
		}
	}

	// collect which cores appear in the data
	cores := map[uint8]bool{}
	for ct := range seen {
		cores[ct.core] = true
	}

	// add process metadata (one per core)
	for core := range cores {
		out = append(out, perfettoEvent{
			Name: "process_name",
			Ph:   "M",
			Pid:  uint32(core),
			Args: map[string]any{
				"name": fmt.Sprintf("Core %d", core),
			},
		})
	}

	// add thread metadata for core+task pairs with execution events
	for ct := range seen {
		name := tasks[ct.tid]
		if name == "" {
			name = fmt.Sprintf("task-%d", ct.tid)
		}
		out = append(out, perfettoEvent{
			Name: "thread_name",
			Ph:   "M",
			Pid:  uint32(ct.core),
			Tid:  makeTid(ct.core, ct.tid),
			Args: map[string]any{
				"name": name,
			},
		})
	}

	// collect unique category+name pairs used by spans and events
	type catName struct {
		cat  uint8
		name uint8
	}
	catNames := map[catName]bool{}
	for _, e := range events {
		if e.Type == TraceInstant || e.Type == TraceBegin || e.Type == TraceValue {
			catNames[catName{e.Cat, e.Name}] = true
		}
	}

	// add process metadata for each category
	cats := map[uint8]bool{}
	for cn := range catNames {
		if !cats[cn.cat] {
			cats[cn.cat] = true
			name := labels[cn.cat]
			if name == "" {
				name = fmt.Sprintf("label-%d", cn.cat)
			}
			out = append(out, perfettoEvent{
				Name: "process_name",
				Ph:   "M",
				Pid:  10 + uint32(cn.cat),
				Args: map[string]any{
					"name": name,
				},
			})
		}

		// add thread metadata for each name within a category
		name := labels[cn.name]
		if name == "" {
			name = fmt.Sprintf("label-%d", cn.name)
		}
		out = append(out, perfettoEvent{
			Name: "thread_name",
			Ph:   "M",
			Pid:  10 + uint32(cn.cat),
			Tid:  uint32(cn.name),
			Args: map[string]any{
				"name": name,
			},
		})
	}

	// track open spans for END matching
	type spanInfo struct {
		cat  uint8
		name uint8
	}
	openSpans := map[uint8]spanInfo{}

	// convert events to Perfetto spans and markers
	type pending struct {
		ts   uint32
		task uint8
	}
	last := map[uint8]*pending{}

	for _, e := range events {
		switch e.Type {
		case TraceTaskSwitch:
			// close previous execution span on this core
			if p, ok := last[e.Core]; ok && e.Timestamp > p.ts {
				name := tasks[p.task]
				if name == "" {
					name = fmt.Sprintf("task-%d", p.task)
				}
				out = append(out, perfettoEvent{
					Name: name,
					Cat:  "task",
					Ph:   "X",
					Ts:   p.ts,
					Dur:  e.Timestamp - p.ts,
					Pid:  uint32(e.Core),
					Tid:  makeTid(e.Core, p.task),
				})
			}
			last[e.Core] = &pending{ts: e.Timestamp, task: e.Task}

		case TraceBegin:
			openSpans[e.Span] = spanInfo{cat: e.Cat, name: e.Name}
			name := labels[e.Name]
			if name == "" {
				name = fmt.Sprintf("label-%d", e.Name)
			}
			ev := perfettoEvent{
				Name: name,
				Cat:  labels[e.Cat],
				Ph:   "B",
				Ts:   e.Timestamp,
				Pid:  10 + uint32(e.Cat),
				Tid:  uint32(e.Name),
			}
			if e.Arg != 0 {
				ev.Args = map[string]any{"arg": e.Arg}
			}
			out = append(out, ev)

		case TraceEnd:
			if info, ok := openSpans[e.Span]; ok {
				name := labels[info.name]
				if name == "" {
					name = fmt.Sprintf("label-%d", info.name)
				}
				out = append(out, perfettoEvent{
					Name: name,
					Cat:  labels[info.cat],
					Ph:   "E",
					Ts:   e.Timestamp,
					Pid:  10 + uint32(info.cat),
					Tid:  uint32(info.name),
				})
				delete(openSpans, e.Span)
			}

		case TraceInstant:
			name := labels[e.Name]
			if name == "" {
				name = fmt.Sprintf("label-%d", e.Name)
			}
			ev := perfettoEvent{
				Name: name,
				Cat:  labels[e.Cat],
				Ph:   "i",
				Ts:   e.Timestamp,
				Pid:  10 + uint32(e.Cat),
				Tid:  uint32(e.Name),
			}
			if e.Arg != 0 {
				ev.Args = map[string]any{"arg": e.Arg}
			}
			out = append(out, ev)

		case TraceValue:
			name := labels[e.Name]
			if name == "" {
				name = fmt.Sprintf("label-%d", e.Name)
			}
			out = append(out, perfettoEvent{
				Name: name,
				Ph:   "C",
				Ts:   e.Timestamp,
				Pid:  10 + uint32(e.Cat),
				Tid:  uint32(e.Name),
				Args: map[string]any{
					name: e.Value,
				},
			})
		}
	}

	return json.MarshalIndent(out, "", "  ")
}
