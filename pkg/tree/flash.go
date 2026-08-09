package tree

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// flashStep is a single esptool invocation performed while flashing.
type flashStep struct {
	message string
	args    []string
}

// flashSteps will assemble the esptool invocations needed to flash the project.
// All offsets and flash settings are taken from the flasher arguments written
// by the build, as they depend on the target and the partition table.
func flashSteps(buildDir, espTool, port, baudRate string, args *flasherArgs, erase, appOnly bool) ([]flashStep, error) {
	// prepare the arguments shared by all invocations
	common := []string{espTool}
	if args.Extra.Chip != "" {
		common = append(common, "--chip", args.Extra.Chip)
	}
	common = append(common, "--port", port, "--baud", baudRate)
	common = append(common, "--before", orDefault(args.Extra.Before, "default_reset"))
	common = append(common, "--after", orDefault(args.Extra.After, "hard_reset"))

	// prepare command builder that keeps the common arguments intact
	command := func(arg ...string) []string {
		return append(append([]string{}, common...), arg...)
	}

	// prepare write command builder
	write := func(items ...flasherArgsItem) []string {
		arg := []string{"write_flash", "-z"}
		arg = append(arg, args.WriteFlashArgs...)
		for _, item := range items {
			arg = append(arg, item.Offset, filepath.Join(buildDir, item.File))
		}
		return command(arg...)
	}

	// prepare steps
	var steps []flashStep

	// erase if requested
	if erase {
		steps = append(steps, flashStep{"Erasing flash...", command("erase_flash")})
	}

	// flash app only
	if appOnly {
		return append(steps, flashStep{"Flashing (app only)...", write(args.Application)}), nil
	}

	// flash all
	steps = append(steps, flashStep{"Flashing...", write(args.Bootloader, args.PartitionTable, args.Application)})

	// erase ota config if not already erased, to ensure the flashed app is
	// selected on the next boot
	if !erase && args.OTAData.Offset != "" {
		// get the size of the otadata partition from its initial image
		stat, err := os.Stat(filepath.Join(buildDir, args.OTAData.File))
		if err != nil {
			return nil, fmt.Errorf("failed to stat OTA data binary: %w", err)
		}

		// erase region
		size := fmt.Sprintf("0x%x", stat.Size())
		steps = append(steps, flashStep{"Erasing OTA config...", command("erase_region", args.OTAData.Offset, size)})
	}

	return steps, nil
}

// Flash will flash the project using the specified serial port.
func Flash(naosPath, port, baudRate string, erase, appOnly, alt bool, out io.Writer) error {
	// read flasher arguments
	args, err := readFlasherArgs(naosPath)
	if err != nil {
		return err
	}

	// calculate paths
	buildDir := filepath.Join(Directory(naosPath), "build")
	espTool := filepath.Join(IDFDirectory(naosPath), "components", "esptool_py", "esptool", "esptool.py")
	if alt {
		espTool, err = exec.LookPath("esptool.py")
		if err != nil {
			return err
		}
	}

	// assemble steps
	steps, err := flashSteps(buildDir, espTool, port, baudRate, args, erase, appOnly)
	if err != nil {
		return err
	}

	// run steps, invoking the alternative esptool directly and letting its
	// shebang pick the interpreter, as it may live in its own environment
	for _, step := range steps {
		utils.Log(out, step.message)
		program, arg := "python", step.args
		if alt {
			program, arg = step.args[0], step.args[1:]
		}
		err = Exec(naosPath, out, nil, alt, false, program, arg...)
		if err != nil {
			return err
		}
	}

	return nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
