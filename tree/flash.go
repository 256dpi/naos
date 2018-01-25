package tree

import (
	"io"
	"path/filepath"

	"github.com/shiftr-io/naos/utils"
)

// Flash will flash the project using the specified serial port.
func Flash(treePath, port string, erase, appOnly bool, out io.Writer) error {
	// calculate paths
	espTool := filepath.Join(IDFDirectory(treePath), "components", "esptool_py", "esptool", "esptool.py")
	bootLoaderBinary := filepath.Join(treePath, "build", "bootloader", "bootloader.bin")
	projectBinary := filepath.Join(treePath, "build", "naos-project.bin")
	partitionsBinary := filepath.Join(treePath, "build", "partitions.bin")

	// prepare erase flash command
	eraseFlash := []string{
		espTool,
		"--chip", "esp32",
		"--port", port,
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"erase_flash",
	}

	// prepare flash all command
	flashAll := []string{
		espTool,
		"--chip", "esp32",
		"--port", port,
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash",
		"-z",
		"--flash_mode", "dio",
		"--flash_freq", "40m",
		"--flash_size", "detect",
		"0x1000", bootLoaderBinary,
		"0x10000", projectBinary,
		"0x8000", partitionsBinary,
	}

	// prepare flash app command
	flashApp := []string{
		espTool,
		"--chip", "esp32",
		"--port", port,
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash",
		"-z",
		"--flash_mode", "dio",
		"--flash_freq", "40m",
		"--flash_size", "detect",
		"0x10000", projectBinary,
	}

	// erase if requested
	if erase {
		utils.Log(out, "Erasing flash...")
		err := Exec(treePath, out, nil, "python", eraseFlash...)
		if err != nil {
			return err
		}
	}

	// flash app only
	if appOnly {
		utils.Log(out, "Flashing (app only)...")
		err := Exec(treePath, out, nil, "python", flashApp...)
		if err != nil {
			return err
		}

		return nil
	}

	// flash all
	utils.Log(out, "Flashing...")
	err := Exec(treePath, out, nil, "python", flashAll...)
	if err != nil {
		return err
	}

	return nil
}
