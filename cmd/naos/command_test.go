package main

import (
	"io"
	"testing"

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
	table := []struct {
		args []string
		cmd  command
	}{
		{[]string{"create"}, command{name: "create"}},
		{[]string{"create", "--cmake", "--force"}, command{name: "create", oCMake: true, oForce: true}},
		{[]string{"install"}, command{name: "install"}},
		{[]string{"install", "--force"}, command{name: "install", oForce: true}},
		{[]string{"build"}, command{name: "build"}},
		{[]string{"build", "--clean", "--reconfigure", "--app-only"}, command{name: "build", oClean: true, oReconfigure: true, oAppOnly: true}},
		{[]string{"detect"}, command{name: "detect"}},
		{[]string{"flash"}, command{name: "flash"}},
		{[]string{"flash", "/dev/x", "--baud=115200", "--erase", "--app-only", "--alt"}, command{name: "flash", aDevice: "/dev/x", oBaudRate: "115200", oErase: true, oAppOnly: true, oAlt: true}},
		{[]string{"flash", "--baud", "115200", "/dev/x"}, command{name: "flash", aDevice: "/dev/x", oBaudRate: "115200"}},
		{[]string{"flash", "-b", "115200"}, command{name: "flash", oBaudRate: "115200"}},
		{[]string{"attach"}, command{name: "attach"}},
		{[]string{"attach", "/dev/x", "--no-reset"}, command{name: "attach", aDevice: "/dev/x", oNoReset: true}},
		{[]string{"run", "/dev/x", "--clean", "--reconfigure", "--app-only", "--baud=115200", "--erase", "--alt"}, command{name: "run", aDevice: "/dev/x", oClean: true, oReconfigure: true, oAppOnly: true, oBaudRate: "115200", oErase: true, oAlt: true}},
		{[]string{"exec", "idf.py"}, command{name: "exec", aCommand: "idf.py", aArgs: []string{}}},
		{[]string{"exec", "idf.py", "size", "--json"}, command{name: "exec", aCommand: "idf.py", aArgs: []string{"size", "--json"}}},
		{[]string{"exec", "--", "idf.py", "--version"}, command{name: "exec", aCommand: "idf.py", aArgs: []string{"--version"}}},
		{[]string{"config", "--read", "params.json"}, command{name: "config", oRead: true, aFile: "params.json"}},
		{[]string{"config", "--write", "params.json", "/dev/x", "--baud=115200"}, command{name: "config", oWrite: true, aFile: "params.json", aDevice: "/dev/x", oBaudRate: "115200"}},
		{[]string{"format"}, command{name: "format"}},
		{[]string{"bundle"}, command{name: "bundle"}},
		{[]string{"bundle", "out.zip", "--add-debug"}, command{name: "bundle", aFile: "out.zip", oAddDebug: true}},
		{[]string{"debug"}, command{name: "debug"}},
		{[]string{"debug", "core.bin", "--elf=app.elf"}, command{name: "debug", aFile: "core.bin", oELF: "app.elf"}},
		{[]string{"sdks"}, command{name: "sdks"}},
		{[]string{"sdks", "--remove=v5.5.5"}, command{name: "sdks", oRemove: "v5.5.5"}},
		{[]string{}, command{}},
		{[]string{"help"}, command{}},
		{[]string{"flash", "--help"}, command{}},
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
		{"build", "extra"},
		{"build", "--unknown"},
		{"flash", "/dev/x", "/dev/y"},
		{"exec"},
		{"config", "params.json"},
		{"config", "--read", "--write", "params.json"},
		{"config", "--read"},
		{"bundle", "a.zip", "b.zip"},
	}

	for _, args := range table {
		_, err := parseTest(args...)
		assert.Error(t, err, args)
	}
}
