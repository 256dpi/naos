package tree

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"tinygo.org/x/espflasher/pkg/espflasher"

	"github.com/256dpi/naos/pkg/utils"
)

// flashStepKind describes the operation performed by a flash step.
type flashStepKind int

const (
	// flashWrite writes the images of the step.
	flashWrite flashStepKind = iota

	// flashErase erases the whole flash.
	flashErase

	// flashEraseRegion erases the region of the step.
	flashEraseRegion
)

// flashImage is a single image written to flash.
type flashImage struct {
	offset uint32
	file   string
}

// flashStep is a single operation performed while flashing.
type flashStep struct {
	message string
	kind    flashStepKind
	images  []flashImage
	offset  uint32
	size    uint32
}

// flashSteps will assemble the steps needed to flash the project. All offsets
// and flash settings are taken from the flasher arguments written by the build,
// as they depend on the target and the partition table.
func flashSteps(buildDir string, args *flasherArgs, erase, appOnly bool) ([]flashStep, error) {
	// prepare image builder
	images := func(items ...flasherArgsItem) ([]flashImage, error) {
		list := make([]flashImage, 0, len(items))
		for _, item := range items {
			offset, err := parseOffset(item.Offset)
			if err != nil {
				return nil, err
			}
			list = append(list, flashImage{
				offset: offset,
				file:   filepath.Join(buildDir, item.File),
			})
		}
		return list, nil
	}

	// prepare steps
	var steps []flashStep

	// erase if requested
	if erase {
		steps = append(steps, flashStep{message: "Erasing flash...", kind: flashErase})
	}

	// flash app only
	if appOnly {
		list, err := images(args.Application)
		if err != nil {
			return nil, err
		}
		return append(steps, flashStep{
			message: "Flashing (app only)...",
			kind:    flashWrite,
			images:  list,
		}), nil
	}

	// flash all
	list, err := images(args.Bootloader, args.PartitionTable, args.Application)
	if err != nil {
		return nil, err
	}
	steps = append(steps, flashStep{
		message: "Flashing...",
		kind:    flashWrite,
		images:  list,
	})

	// erase ota config if not already erased, to ensure the flashed app is
	// selected on the next boot
	if !erase && args.OTAData.Offset != "" {
		// get the size of the otadata partition from its initial image
		stat, err := os.Stat(filepath.Join(buildDir, args.OTAData.File))
		if err != nil {
			return nil, fmt.Errorf("failed to stat OTA data binary: %w", err)
		}

		// parse offset
		offset, err := parseOffset(args.OTAData.Offset)
		if err != nil {
			return nil, err
		}

		// erase region
		steps = append(steps, flashStep{
			message: "Erasing OTA config...",
			kind:    flashEraseRegion,
			offset:  offset,
			size:    uint32(stat.Size()),
		})
	}

	return steps, nil
}

// parseOffset will parse a flash offset as written by the build.
func parseOffset(str string) (uint32, error) {
	value, err := strconv.ParseUint(str, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid offset %q: %w", str, err)
	}

	return uint32(value), nil
}

// Flash will flash the project using the specified serial port.
func Flash(naosPath, port, baudRate string, erase, appOnly, alt bool, out io.Writer) error {
	// read flasher arguments
	args, err := readFlasherArgs(naosPath)
	if err != nil {
		return err
	}

	// assemble steps
	buildDir := filepath.Join(Directory(naosPath), "build")
	steps, err := flashSteps(buildDir, args, erase, appOnly)
	if err != nil {
		return err
	}

	// use external esptool if requested
	if alt {
		return flashWithESPTool(naosPath, steps, port, baudRate, args, out)
	}

	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := connectDevice(port, baudRate, args, out)
	if err != nil {
		return err
	}
	defer flasher.Close()

	// reset device on return, to leave it running the flashed application
	if orDefault(args.Extra.After, "hard_reset") == "hard_reset" {
		defer flasher.Reset()
	}

	// verify chip, as an image built for another target does not boot
	err = verifyChip(flasher.ChipName(), args.Extra.Chip)
	if err != nil {
		return err
	}

	// run steps
	for _, step := range steps {
		utils.Log(out, step.message)
		err = runFlashStep(flasher, step, out)
		if err != nil {
			return err
		}
	}

	return nil
}

// runFlashStep will perform the provided step.
func runFlashStep(flasher *espflasher.Flasher, step flashStep, out io.Writer) error {
	switch step.kind {
	case flashErase:
		return flasher.EraseFlash(progressLogger(out))
	case flashEraseRegion:
		return flasher.EraseRegion(step.offset, step.size, progressLogger(out))
	default:
		// read images
		parts := make([]espflasher.ImagePart, 0, len(step.images))
		for _, image := range step.images {
			data, err := os.ReadFile(image.file)
			if err != nil {
				return err
			}
			parts = append(parts, espflasher.ImagePart{
				Data:   data,
				Offset: image.offset,
			})
		}

		return flasher.FlashImages(parts, progressLogger(out))
	}
}

// verifyChip will check that the detected chip matches the built target.
func verifyChip(detected, chip string) error {
	// skip if unknown
	if chip == "" {
		return nil
	}

	// normalize name, dropping an eventual revision suffix
	name := strings.ToLower(strings.ReplaceAll(detected, "-", ""))
	name, _, _ = strings.Cut(name, "rev")

	// compare name
	if name != strings.ToLower(chip) {
		return fmt.Errorf("connected chip %s does not match target %q", detected, chip)
	}

	return nil
}

// flashWithESPTool will perform the steps using the esptool found in the path,
// which may support targets or features the built-in flasher does not.
func flashWithESPTool(naosPath string, steps []flashStep, port, baudRate string, args *flasherArgs, out io.Writer) error {
	// find esptool
	espTool, err := exec.LookPath("esptool.py")
	if err != nil {
		return err
	}

	// run steps, invoking esptool directly and letting its shebang pick the
	// interpreter, as it may live in its own environment
	for _, step := range steps {
		utils.Log(out, step.message)
		err = Exec(naosPath, out, nil, true, false, espTool, espToolArgs(step, port, baudRate, args)...)
		if err != nil {
			return err
		}
	}

	return nil
}

// espToolArgs will assemble the esptool arguments for the provided step.
func espToolArgs(step flashStep, port, baudRate string, args *flasherArgs) []string {
	// prepare the arguments shared by all invocations
	var arg []string
	if args.Extra.Chip != "" {
		arg = append(arg, "--chip", args.Extra.Chip)
	}
	arg = append(arg, "--port", port, "--baud", baudRate)
	arg = append(arg, "--before", orDefault(args.Extra.Before, "default_reset"))
	arg = append(arg, "--after", orDefault(args.Extra.After, "hard_reset"))

	// add command
	switch step.kind {
	case flashErase:
		arg = append(arg, "erase_flash")
	case flashEraseRegion:
		arg = append(arg, "erase_region", fmt.Sprintf("0x%x", step.offset), fmt.Sprintf("0x%x", step.size))
	default:
		arg = append(arg, "write_flash", "-z")
		arg = append(arg, args.WriteFlashArgs...)
		for _, image := range step.images {
			arg = append(arg, fmt.Sprintf("0x%x", image.offset), image.file)
		}
	}

	return arg
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
