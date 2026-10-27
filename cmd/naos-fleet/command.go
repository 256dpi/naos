package main

import (
	"time"

	"github.com/spf13/cobra"
)

const header = `Networked Artifacts Operating System
© Joël Gähwiler
https://github.com/256dpi/naos`

type command struct {
	// command
	name string

	// arguments
	aParam   string
	aPattern string
	aValue   string
	aVersion string
	aFile    string

	// options
	oBroker   string
	oHub      string
	oClear    bool
	oDelete   bool
	oDuration time.Duration
	oJobs     int
}

func parseCommand(args []string) (*command, error) {
	// prepare command
	cmd := &command{}
	root := newRootCommand(cmd)
	root.SetArgs(args)

	// parse arguments, cobra reports errors itself
	err := root.Execute()

	return cmd, err
}

func newRootCommand(cmd *command) *cobra.Command {
	// keep the declaration order in the help
	cobra.EnableCommandSorting = false

	// create root
	root := &cobra.Command{
		Use:  "naos-fleet",
		Long: header,
	}

	// add groups
	root.AddGroup(
		&cobra.Group{ID: "fleet", Title: "Fleet Management:"},
		&cobra.Group{ID: "utilities", Title: "Utilities:"},
	)
	root.SetHelpCommandGroupID("utilities")
	root.SetCompletionCommandGroupID("utilities")

	// prepare helper that registers a command and records its name and
	// positional arguments when it is selected
	add := func(c *cobra.Command, args func([]string)) *cobra.Command {
		c.GroupID = "fleet"
		c.Run = func(_ *cobra.Command, list []string) {
			cmd.name = c.Name()
			if args != nil {
				args(list)
			}
		}
		root.AddCommand(c)
		return c
	}

	// prepare helper that returns an optional positional argument
	optional := func(list []string, index int) string {
		if len(list) > index {
			return list[index]
		}
		return ""
	}

	// prepare flag helper shared by several commands
	jobs := func(c *cobra.Command) {
		c.Flags().IntVarP(&cmd.oJobs, "jobs", "j", 10, "Number of simultaneous jobs")
	}
	pattern := func(list []string) {
		cmd.aPattern = optional(list, 0)
	}

	create := add(&cobra.Command{
		Use:   "create",
		Short: "Create a new fleet file",
		Args:  cobra.NoArgs,
	}, nil)
	create.Flags().StringVarP(&cmd.oBroker, "broker", "b", "", "MQTT broker `url` of the new fleet")
	create.Flags().StringVar(&cmd.oHub, "hub", "", "NAOS hub base `url` of the new fleet")

	add(&cobra.Command{
		Use:   "list",
		Short: "List all devices listed in the fleet",
		Args:  cobra.NoArgs,
	}, nil)

	collect := add(&cobra.Command{
		Use:   "collect",
		Short: "Collect devices and add them to the fleet",
		Args:  cobra.NoArgs,
	}, nil)
	collect.Flags().BoolVar(&cmd.oClear, "clear", false, "Remove not available devices from fleet")
	collect.Flags().DurationVarP(&cmd.oDuration, "duration", "d", 5*time.Second, "Operation `duration`")

	discover := add(&cobra.Command{
		Use:   "discover [<pattern>]",
		Short: "Discover parameters and metrics of a device",
		Args:  cobra.MaximumNArgs(1),
	}, pattern)
	jobs(discover)

	ping := add(&cobra.Command{
		Use:   "ping [<pattern>]",
		Short: "Ping devices",
		Args:  cobra.MaximumNArgs(1),
	}, pattern)
	jobs(ping)

	get := add(&cobra.Command{
		Use:   "get <param> [<pattern>]",
		Short: "Read a parameter from devices",
		Args:  cobra.RangeArgs(1, 2),
	}, func(list []string) {
		cmd.aParam = list[0]
		cmd.aPattern = optional(list, 1)
	})
	jobs(get)

	set := add(&cobra.Command{
		Use:   "set <param> [--] <value> [<pattern>]",
		Short: "Set a parameter on devices",
		Args:  cobra.RangeArgs(2, 3),
	}, func(list []string) {
		cmd.aParam = list[0]
		cmd.aValue = list[1]
		cmd.aPattern = optional(list, 2)
	})
	jobs(set)

	unset := add(&cobra.Command{
		Use:   "unset <param> [<pattern>]",
		Short: "Unset a parameter on devices",
		Args:  cobra.RangeArgs(1, 2),
	}, func(list []string) {
		cmd.aParam = list[0]
		cmd.aPattern = optional(list, 1)
	})
	jobs(unset)

	add(&cobra.Command{
		Use:   "monitor [<pattern>]",
		Short: "Monitor system information from devices",
		Args:  cobra.MaximumNArgs(1),
	}, pattern)

	add(&cobra.Command{
		Use:   "record [<pattern>]",
		Short: "Record log messages from devices",
		Args:  cobra.MaximumNArgs(1),
	}, pattern)

	debug := add(&cobra.Command{
		Use:   "debug [<pattern>]",
		Short: "Gather coredump from devices",
		Args:  cobra.MaximumNArgs(1),
	}, pattern)
	debug.Flags().BoolVar(&cmd.oDelete, "delete", false, "Delete loaded coredumps from the devices")
	jobs(debug)

	update := add(&cobra.Command{
		Use:   "update <version> <file> [<pattern>]",
		Short: "Update a firmware to devices",
		Args:  cobra.RangeArgs(2, 3),
	}, func(list []string) {
		cmd.aVersion = list[0]
		cmd.aFile = list[1]
		cmd.aPattern = optional(list, 2)
	})
	jobs(update)

	return root
}
