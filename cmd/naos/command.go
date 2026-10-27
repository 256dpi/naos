package main

import (
	"github.com/spf13/cobra"
)

const header = `Networked Artifacts Operating System
© Joël Gähwiler
https://github.com/256dpi/naos`

type command struct {
	// command
	name string

	// arguments
	aDevice  string
	aFile    string
	aCommand string
	aArgs    []string

	// options
	oForce       bool
	oBaudRate    string
	oCMake       bool
	oClean       bool
	oReconfigure bool
	oErase       bool
	oAppOnly     bool
	oAlt         bool
	oAddDebug    bool
	oNoReset     bool
	oRead        bool
	oWrite       bool
	oELF         string
	oRemove      string
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
		Use:  "naos",
		Long: header,
	}

	// add groups
	root.AddGroup(
		&cobra.Group{ID: "project", Title: "Project Management:"},
		&cobra.Group{ID: "utilities", Title: "Utilities:"},
	)
	root.SetHelpCommandGroupID("utilities")
	root.SetCompletionCommandGroupID("utilities")

	// prepare helper that registers a command and records its name and
	// positional arguments when it is selected
	add := func(group string, c *cobra.Command, args func([]string)) *cobra.Command {
		c.GroupID = group
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

	// prepare flag helpers shared by several commands
	baud := func(c *cobra.Command) {
		c.Flags().StringVarP(&cmd.oBaudRate, "baud", "b", "", "The baud `rate`")
	}
	building := func(c *cobra.Command) {
		c.Flags().BoolVar(&cmd.oClean, "clean", false, "Clean all build artifacts before building again")
		c.Flags().BoolVar(&cmd.oReconfigure, "reconfigure", false, "Reconfigure will recalculate the sdkconfig file")
		c.Flags().BoolVar(&cmd.oAppOnly, "app-only", false, "Only build or flash the application")
	}
	flashing := func(c *cobra.Command) {
		baud(c)
		c.Flags().BoolVar(&cmd.oErase, "erase", false, "Erase completely before flashing new image")
		c.Flags().BoolVar(&cmd.oAlt, "alt", false, "Use alternative esptool.py found in PATH")
	}
	device := func(list []string) {
		cmd.aDevice = optional(list, 0)
	}

	/* project management */

	create := add("project", &cobra.Command{
		Use:   "create",
		Short: "Create a new naos project in the current directory",
		Args:  cobra.NoArgs,
	}, nil)
	create.Flags().BoolVar(&cmd.oCMake, "cmake", false, "Create required CMake files for IDEs like CLion")
	create.Flags().BoolVar(&cmd.oForce, "force", false, "Create a new manifest when one already exists")

	install := add("project", &cobra.Command{
		Use:   "install",
		Short: "Download required dependencies to the 'naos' subdirectory",
		Args:  cobra.NoArgs,
	}, nil)
	install.Flags().BoolVar(&cmd.oForce, "force", false, "Reinstall dependencies when they already exist")

	build := add("project", &cobra.Command{
		Use:   "build",
		Short: "Build all source files",
		Args:  cobra.NoArgs,
	}, nil)
	building(build)

	add("project", &cobra.Command{
		Use:   "detect",
		Short: "Detect all connected devices",
		Args:  cobra.NoArgs,
	}, nil)

	flash := add("project", &cobra.Command{
		Use:   "flash [<device>]",
		Short: "Flash the previously built binary to an attached device",
		Args:  cobra.MaximumNArgs(1),
	}, device)
	flash.Flags().BoolVar(&cmd.oAppOnly, "app-only", false, "Only flash the application")
	flashing(flash)

	attach := add("project", &cobra.Command{
		Use:   "attach [<device>]",
		Short: "Open a serial communication with an attached device",
		Args:  cobra.MaximumNArgs(1),
	}, device)
	attach.Flags().BoolVar(&cmd.oNoReset, "no-reset", false, "Do not reset the device when attaching")

	run := add("project", &cobra.Command{
		Use:   "run [<device>]",
		Short: "Run 'build', 'flash', and 'attach' sequentially",
		Args:  cobra.MaximumNArgs(1),
	}, device)
	building(run)
	flashing(run)

	exec := add("project", &cobra.Command{
		Use:   "exec [--] <command> [<args>...]",
		Short: "Run a command in the tree",
		Args:  cobra.MinimumNArgs(1),
	}, func(list []string) {
		cmd.aCommand = list[0]
		cmd.aArgs = list[1:]
	})
	exec.Flags().SetInterspersed(false)

	config := add("project", &cobra.Command{
		Use:   "config (--read | --write) <file> [<device>]",
		Short: "Write parameters to or read them from an attached device",
		Args:  cobra.RangeArgs(1, 2),
	}, func(list []string) {
		cmd.aFile = list[0]
		cmd.aDevice = optional(list, 1)
	})
	config.Flags().BoolVar(&cmd.oRead, "read", false, "Read the parameters from the device into the file")
	config.Flags().BoolVar(&cmd.oWrite, "write", false, "Write the parameters from the file to the device")
	config.MarkFlagsOneRequired("read", "write")
	config.MarkFlagsMutuallyExclusive("read", "write")
	baud(config)

	add("project", &cobra.Command{
		Use:   "format",
		Short: "Format all source files in the 'src' subdirectory",
		Args:  cobra.NoArgs,
	}, nil)

	bundle := add("project", &cobra.Command{
		Use:   "bundle [<file>]",
		Short: "Generate a bundle of the project",
		Args:  cobra.MaximumNArgs(1),
	}, func(list []string) {
		cmd.aFile = optional(list, 0)
	})
	bundle.Flags().BoolVar(&cmd.oAddDebug, "add-debug", false, "Add debug ELF file to bundle")

	/* utilities */

	sdks := add("utilities", &cobra.Command{
		Use:   "sdks",
		Short: "List installed SDKs with their versions and location",
		Args:  cobra.NoArgs,
	}, nil)
	sdks.Flags().StringVar(&cmd.oRemove, "remove", "", "Remove the SDKs installed for the specified `version`")

	debug := add("utilities", &cobra.Command{
		Use:   "debug [<file>]",
		Short: "Parse and print a coredump analysis",
		Args:  cobra.MaximumNArgs(1),
	}, func(list []string) {
		cmd.aFile = optional(list, 0)
	})
	debug.Flags().StringVar(&cmd.oELF, "elf", "", "The ELF `file` for coredump analysis")

	return root
}
