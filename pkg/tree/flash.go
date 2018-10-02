package tree

import (
	"io"
	"path/filepath"

	"github.com/shiftr-io/naos/pkg/utils"
)

// Flash will flash the project using the specified serial port.
func Flash(naosPath, port string, erase, appOnly bool, out io.Writer) error {
	// calculate paths
	espTool := filepath.Join(IDFDirectory(naosPath), "components", "esptool_py", "esptool", "esptool.py")
	bootLoaderBinary := filepath.Join(Directory(naosPath), "build", "bootloader", "bootloader.bin")
	projectBinary := filepath.Join(Directory(naosPath), "build", "naos-project.bin")
	partitionsBinary := filepath.Join(Directory(naosPath), "build", "partitions.bin")

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

	// prepare erase ota command
	eraseOTA := []string{
		espTool,
		"--chip", "esp32",
		"--port", port,
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"erase_region", "0xd000", "0x2000",
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
		"0x8000", partitionsBinary,
		"0x10000", projectBinary,
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
		err := Exec(naosPath, out, nil, "python", eraseFlash...)
		if err != nil {
			return err
		}
	}

	// flash app only
	if appOnly {
		utils.Log(out, "Flashing (app only)...")
		err := Exec(naosPath, out, nil, "python", flashApp...)
		if err != nil {
			return err
		}

		// erase ota if not already erased
		if !erase {
			utils.Log(out, "Erasing OTA config...")
			err := Exec(naosPath, out, nil, "python", eraseOTA...)
			if err != nil {
				return err
			}
		}

		return nil
	}

	// flash all
	utils.Log(out, "Flashing...")
	err := Exec(naosPath, out, nil, "python", flashAll...)
	if err != nil {
		return err
	}

	// erase ota if not already erased
	if !erase {
		utils.Log(out, "Erasing OTA config...")
		err := Exec(naosPath, out, nil, "python", eraseOTA...)
		if err != nil {
			return err
		}
	}

	return nil
}
