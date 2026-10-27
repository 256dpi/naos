package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v2"

	"github.com/256dpi/naos/pkg/msg"
	"github.com/256dpi/naos/pkg/serial"
	"github.com/256dpi/naos/pkg/utils"
)

const configTimeout = 5 * time.Second

func configList(cmd *command) {
	exitIfSet(withSession(cmd.aDevice, cmd.oPassword, func(s *msg.Session) error {
		// read parameters
		infos, values, err := readParams(s)
		if err != nil {
			return err
		}

		// prepare table
		tbl := newTable("NAME", "TYPE", "MODE", "VALUE")

		// add rows
		for _, info := range infos {
			tbl.add(info.Name, typeName(info.Type), modeName(info.Mode), printable(values[info.Ref]))
		}

		// show table
		tbl.show(0)

		return nil
	}))
}

func configGet(cmd *command) {
	exitIfSet(withSession(cmd.aDevice, cmd.oPassword, func(s *msg.Session) error {
		// find parameter
		info, err := findParam(s, cmd.aParam)
		if err != nil {
			return err
		}

		// check parameter
		if info.Type == msg.ParamTypeAction {
			return fmt.Errorf("parameter %q is an action", info.Name)
		}

		// read value
		value, err := msg.ReadParam(s, info.Ref, configTimeout)
		if err != nil {
			return err
		}

		// print value
		fmt.Println(printable(value))

		return nil
	}))
}

func configSet(cmd *command) {
	exitIfSet(withSession(cmd.aDevice, cmd.oPassword, func(s *msg.Session) error {
		// find parameter
		info, err := findParam(s, cmd.aParam)
		if err != nil {
			return err
		}

		// check parameter
		if !writable(info) {
			return fmt.Errorf("parameter %q is not writable", info.Name)
		}

		// set value
		return msg.WriteParam(s, info.Ref, []byte(cmd.aValue), configTimeout)
	}))
}

func configUnset(cmd *command) {
	exitIfSet(withSession(cmd.aDevice, cmd.oPassword, func(s *msg.Session) error {
		// find parameter
		info, err := findParam(s, cmd.aParam)
		if err != nil {
			return err
		}

		// check parameter
		if !writable(info) {
			return fmt.Errorf("parameter %q is not writable", info.Name)
		}

		// clear value, which restores the default
		return msg.ClearParam(s, info.Ref, configTimeout)
	}))
}

func configRead(cmd *command) {
	exitIfSet(withSession(cmd.aDevice, cmd.oPassword, func(s *msg.Session) error {
		// read parameters
		infos, values, err := readParams(s)
		if err != nil {
			return err
		}

		// collect stored values
		stored := map[string]string{}
		for _, info := range infos {
			if stores(info) {
				stored[info.Name] = string(values[info.Ref])
			}
		}

		// marshal values
		data, err := yaml.Marshal(stored)
		if err != nil {
			return err
		}

		// write file
		err = os.WriteFile(cmd.aFile, data, 0644)
		if err != nil {
			return err
		}

		// log result
		utils.Log(os.Stdout, fmt.Sprintf("Read %d parameters into %s.", len(stored), cmd.aFile))

		return nil
	}))
}

func configWrite(cmd *command) {
	// read file
	data, err := os.ReadFile(cmd.aFile)
	exitIfSet(err)

	// unmarshal values
	var desired map[string]string
	exitIfSet(yaml.Unmarshal(data, &desired))

	exitIfSet(withSession(cmd.aDevice, cmd.oPassword, func(s *msg.Session) error {
		// read parameters
		infos, values, err := readParams(s)
		if err != nil {
			return err
		}

		// plan changes
		plan := planWrite(infos, values, desired)

		// apply changes
		for _, change := range plan.changes {
			err = msg.WriteParam(s, change.info.Ref, []byte(change.value), configTimeout)
			if err != nil {
				return fmt.Errorf("write %q: %w", change.info.Name, err)
			}
			utils.Log(os.Stdout, fmt.Sprintf("Set %s: %s -> %s", change.info.Name, printable([]byte(change.old)), printable([]byte(change.value))))
		}

		// report other parameters
		for _, name := range plan.skipped {
			utils.Log(os.Stdout, fmt.Sprintf("Skipped %s: not a stored, writable parameter.", name))
		}
		for _, name := range plan.unknown {
			utils.Log(os.Stdout, fmt.Sprintf("Skipped %s: unknown to the device.", name))
		}

		// log summary
		utils.Log(os.Stdout, fmt.Sprintf("Changed %d, unchanged %d, skipped %d.", len(plan.changes), plan.unchanged, len(plan.skipped)+len(plan.unknown)))

		return nil
	}))
}

type paramChange struct {
	info  msg.ParamInfo
	old   string
	value string
}

type writePlan struct {
	changes   []paramChange
	unchanged int
	skipped   []string
	unknown   []string
}

// planWrite determines the writes needed to apply the desired values. Only
// differing values are written, so that parameters still at their default are
// not pinned to it by restoring a file read earlier.
func planWrite(infos []msg.ParamInfo, values map[uint8][]byte, desired map[string]string) writePlan {
	// index parameters
	index := map[string]msg.ParamInfo{}
	for _, info := range infos {
		index[info.Name] = info
	}

	// sort names
	names := make([]string, 0, len(desired))
	for name := range desired {
		names = append(names, name)
	}
	sort.Strings(names)

	// plan changes
	var plan writePlan
	for _, name := range names {
		info, ok := index[name]
		if !ok {
			plan.unknown = append(plan.unknown, name)
		} else if !stores(info) {
			plan.skipped = append(plan.skipped, name)
		} else if string(values[info.Ref]) == desired[name] {
			plan.unchanged++
		} else {
			plan.changes = append(plan.changes, paramChange{
				info:  info,
				old:   string(values[info.Ref]),
				value: desired[name],
			})
		}
	}

	return plan
}

// withSession runs the function with a session to the specified device, or
// the best serial device, unlocking it with the password if it is locked.
func withSession(device, password string, fn func(s *msg.Session) error) error {
	// get device
	var dev msg.Device
	var err error
	if device != "" {
		dev, err = serial.NewDevice(device)
	} else {
		dev, err = serial.BestDevice()
	}
	if err != nil {
		return err
	}

	// run function
	results := msg.Execute([]msg.Device{dev}, 1, func(_ int, s *msg.Session) (any, error) {
		// get status
		status, err := s.Status(configTimeout)
		if err != nil {
			return nil, err
		}

		// unlock if locked
		if status&msg.StatusLocked != 0 {
			if password == "" {
				return nil, errors.New("device is locked, provide its password with --password")
			}
			ok, err := s.Unlock(password, configTimeout)
			if err != nil {
				return nil, err
			} else if !ok {
				return nil, errors.New("invalid password")
			}
		}

		return nil, fn(s)
	}, nil)

	return results[0].Error
}

// readParams returns all parameters and the values of all but the actions.
func readParams(s *msg.Session) ([]msg.ParamInfo, map[uint8][]byte, error) {
	// list parameters
	infos, err := msg.ListParams(s, configTimeout)
	if err != nil {
		return nil, nil, err
	}

	// read values
	values := map[uint8][]byte{}
	for _, info := range infos {
		if info.Type == msg.ParamTypeAction {
			continue
		}
		value, err := msg.ReadParam(s, info.Ref, configTimeout)
		if err != nil {
			return nil, nil, fmt.Errorf("read %q: %w", info.Name, err)
		}
		values[info.Ref] = value
	}

	return infos, values, nil
}

func findParam(s *msg.Session, name string) (msg.ParamInfo, error) {
	// list parameters
	infos, err := msg.ListParams(s, configTimeout)
	if err != nil {
		return msg.ParamInfo{}, err
	}

	// find parameter
	for _, info := range infos {
		if info.Name == name {
			return info, nil
		}
	}

	return msg.ParamInfo{}, fmt.Errorf("unknown parameter %q", name)
}

// writable returns whether the parameter accepts values over messages: locked
// parameters are read-only, and writing an action would trigger it.
func writable(info msg.ParamInfo) bool {
	return info.Type != msg.ParamTypeAction && info.Mode&msg.ParamModeLocked == 0
}

// stores returns whether the parameter is writable and keeps its value across
// restarts, which is what reading and writing a file covers.
func stores(info msg.ParamInfo) bool {
	return writable(info) && info.Mode&msg.ParamModeVolatile == 0
}

func typeName(typ msg.ParamType) string {
	switch typ {
	case msg.ParamTypeRaw:
		return "raw"
	case msg.ParamTypeString:
		return "string"
	case msg.ParamTypeBool:
		return "bool"
	case msg.ParamTypeLong:
		return "long"
	case msg.ParamTypeDouble:
		return "double"
	case msg.ParamTypeAction:
		return "action"
	default:
		return "unknown"
	}
}

func modeName(mode msg.ParamMode) string {
	var list []string
	if mode&msg.ParamModeSystem != 0 {
		list = append(list, "system")
	}
	if mode&msg.ParamModeApplication != 0 {
		list = append(list, "application")
	}
	if mode&msg.ParamModeVolatile != 0 {
		list = append(list, "volatile")
	}
	if mode&msg.ParamModeLocked != 0 {
		list = append(list, "locked")
	}
	return strings.Join(list, ",")
}

// printable returns the value as text, or hex encoded if it is binary.
func printable(value []byte) string {
	if utf8.Valid(value) && strings.IndexFunc(string(value), func(r rune) bool {
		return unicode.IsControl(r)
	}) < 0 {
		return string(value)
	}
	return fmt.Sprintf("0x%x", value)
}
