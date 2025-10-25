package msg

import (
	"errors"
	"fmt"
	"math"
	"time"
)

const paramsEndpoint = 0x1

// ParamType represents a parameter type.
type ParamType uint8

// The available parameter types.
const (
	ParamTypeRaw ParamType = iota
	ParamTypeString
	ParamTypeBool
	ParamTypeLong
	ParamTypeDouble
	ParamTypeAction
)

// Valid returns whether the parameter type is valid.
func (t ParamType) Valid() bool {
	return t == ParamTypeRaw || t == ParamTypeString || t == ParamTypeBool || t == ParamTypeLong || t == ParamTypeDouble || t == ParamTypeAction
}

// ParamMode represents a parameter mode.
type ParamMode uint8

// The available parameter modes.
const (
	ParamModeVolatile    = ParamMode(1 << 0)
	ParamModeSystem      = ParamMode(1 << 1)
	ParamModeApplication = ParamMode(1 << 2)
	ParamModeLocked      = ParamMode(1 << 4)
)

// Valid returns whether the parameter mode is valid.
func (m ParamMode) Valid() bool {
	return m&^(ParamModeVolatile|ParamModeSystem|ParamModeApplication|ParamModeLocked) == 0
}

// ParamInfo describes a parameter.
type ParamInfo struct {
	Ref  uint8
	Type ParamType
	Mode ParamMode
	Name string
}

// ParamUpdate describes a parameter update.
type ParamUpdate struct {
	Ref   uint8
	Age   uint64
	Value []byte
}

// GetParam returns the value of the named parameter.
func GetParam(s *Session, name string, timeout time.Duration) ([]byte, error) {
	// send command
	cmd := Pack("os", uint8(0), name)
	err := s.Send(paramsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// receive value
	value, err := s.Receive(paramsEndpoint, false, timeout)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// SetParam sets the value of the named parameter.
func SetParam(s *Session, name string, value []byte, timeout time.Duration) error {
	// send command
	cmd := Pack("osob", uint8(1), name, uint8(0), value)
	err := s.Send(paramsEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}

// ListParams returns a list of all parameters.
func ListParams(s *Session, timeout time.Duration) ([]ParamInfo, error) {
	// send command
	cmd := []byte{2}
	err := s.Send(paramsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// prepare list
	var list []ParamInfo

	for {
		// receive reply or return list on ack
		reply, err := s.Receive(paramsEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			break
		} else if err != nil {
			return nil, err
		}

		// verify reply
		if len(reply) < 4 {
			return nil, fmt.Errorf("invalid reply")
		}

		// parse reply
		ref := reply[0]
		typ := ParamType(reply[1])
		mode := ParamMode(reply[2])
		name := string(reply[3:])

		// check type and mode
		if !typ.Valid() || !mode.Valid() {
			return nil, fmt.Errorf("invalid type or mode")
		}

		// append info
		list = append(list, ParamInfo{
			Ref:  ref,
			Type: typ,
			Mode: mode,
			Name: name,
		})
	}

	return list, nil
}

// ReadParam return the value of the referenced parameter.
func ReadParam(s *Session, ref uint8, timeout time.Duration) ([]byte, error) {
	// send command
	cmd := []byte{3, ref}
	err := s.Send(paramsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// receive value
	value, err := s.Receive(paramsEndpoint, false, timeout)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// WriteParam sets the value of the referenced parameter.
func WriteParam(s *Session, ref uint8, value []byte, timeout time.Duration) error {
	// send command
	cmd := Pack("oob", uint8(4), ref, value)
	err := s.Send(paramsEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}

// CollectParams returns a list of parameter updates.
func CollectParams(s *Session, refs []uint8, since uint64, timeout time.Duration) ([]ParamUpdate, error) {
	// prepare map
	var mp uint64 = math.MaxUint64
	if len(refs) > 0 {
		mp = 0
		for _, ref := range refs {
			mp |= 1 << ref
		}
	}

	// send command
	cmd := Pack("oqq", uint8(5), mp, since)
	err := s.Send(paramsEndpoint, cmd, 0)
	if err != nil {
		return nil, err
	}

	// prepare list
	var list []ParamUpdate

	for {
		// receive reply or return list on ack
		reply, err := s.Receive(paramsEndpoint, true, timeout)
		if errors.Is(err, Ack) {
			break
		} else if err != nil {
			return nil, err
		}

		// verify reply
		if len(reply) < 9 {
			return nil, fmt.Errorf("invalid reply")
		}

		// unpack reply
		args := Unpack("oqb", reply)

		// append info
		list = append(list, ParamUpdate{
			Ref:   args[0].(uint8),
			Age:   args[1].(uint64),
			Value: args[2].([]byte),
		})
	}

	return list, nil
}

// ClearParam clears the value of the referenced parameter.
func ClearParam(s *Session, ref uint8, timeout time.Duration) error {
	// send command
	cmd := []byte{6, ref}
	err := s.Send(paramsEndpoint, cmd, timeout)
	if err != nil {
		return err
	}

	return nil
}
