package main

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func parseTest(args ...string) (*command, error) {
	cmd := &command{}
	root := newRootCommand(cmd)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.Execute()
	return cmd, err
}

func TestParseCommand(t *testing.T) {
	// defaults set on every command
	d := 5 * time.Second
	j := 10

	table := []struct {
		args []string
		cmd  command
	}{
		{[]string{"create"}, command{name: "create", oDuration: d, oJobs: j}},
		{[]string{"create", "--broker=mqtt://x", "--hub=https://y"}, command{name: "create", oBroker: "mqtt://x", oHub: "https://y", oDuration: d, oJobs: j}},
		{[]string{"create", "-b", "mqtt://x"}, command{name: "create", oBroker: "mqtt://x", oDuration: d, oJobs: j}},
		{[]string{"list"}, command{name: "list", oDuration: d, oJobs: j}},
		{[]string{"collect"}, command{name: "collect", oDuration: d, oJobs: j}},
		{[]string{"collect", "--clear", "--duration=10s"}, command{name: "collect", oClear: true, oDuration: 10 * time.Second, oJobs: j}},
		{[]string{"collect", "-d", "1m"}, command{name: "collect", oDuration: time.Minute, oJobs: j}},
		{[]string{"discover"}, command{name: "discover", oDuration: d, oJobs: j}},
		{[]string{"discover", "al-*", "--jobs=3"}, command{name: "discover", aPattern: "al-*", oDuration: d, oJobs: 3}},
		{[]string{"ping", "al-*", "-j", "3"}, command{name: "ping", aPattern: "al-*", oDuration: d, oJobs: 3}},
		{[]string{"get", "gas-window"}, command{name: "get", aParam: "gas-window", oDuration: d, oJobs: j}},
		{[]string{"get", "gas-window", "al-*", "--jobs=3"}, command{name: "get", aParam: "gas-window", aPattern: "al-*", oDuration: d, oJobs: 3}},
		{[]string{"set", "gas-window", "600"}, command{name: "set", aParam: "gas-window", aValue: "600", oDuration: d, oJobs: j}},
		{[]string{"set", "gas-window", "--", "-1", "al-*"}, command{name: "set", aParam: "gas-window", aValue: "-1", aPattern: "al-*", oDuration: d, oJobs: j}},
		{[]string{"unset", "gas-window", "al-*"}, command{name: "unset", aParam: "gas-window", aPattern: "al-*", oDuration: d, oJobs: j}},
		{[]string{"monitor", "al-*"}, command{name: "monitor", aPattern: "al-*", oDuration: d, oJobs: j}},
		{[]string{"record"}, command{name: "record", oDuration: d, oJobs: j}},
		{[]string{"debug", "--delete", "--jobs=3"}, command{name: "debug", oDelete: true, oDuration: d, oJobs: 3}},
		{[]string{"update", "v1.0.2", "fw.bin"}, command{name: "update", aVersion: "v1.0.2", aFile: "fw.bin", oDuration: d, oJobs: j}},
		{[]string{"update", "v1.0.2", "fw.bin", "al-*", "-j", "3"}, command{name: "update", aVersion: "v1.0.2", aFile: "fw.bin", aPattern: "al-*", oDuration: d, oJobs: 3}},
		{[]string{}, command{oDuration: d, oJobs: j}},
		{[]string{"help"}, command{oDuration: d, oJobs: j}},
	}

	for _, item := range table {
		cmd, err := parseTest(item.args...)
		assert.NoError(t, err, item.args)
		assert.Equal(t, item.cmd, *cmd, item.args)
	}
}

func TestParseCommandErrors(t *testing.T) {
	table := [][]string{
		{"unknown"},
		{"list", "extra"},
		{"get"},
		{"set", "gas-window"},
		{"set", "gas-window", "-1"},
		{"update", "v1.0.2"},
		{"collect", "--duration=soon"},
		{"ping", "--jobs=many"},
	}

	for _, args := range table {
		_, err := parseTest(args...)
		assert.Error(t, err, args)
	}
}
