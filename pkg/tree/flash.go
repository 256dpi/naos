package tree

import (
	"io"
	"os/exec"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// Flash will flash the project using the specified serial port.
func Flash(naosPath, appName, target, port, baudRate string, erase, appOnly, alt bool, out io.Writer) error {
	// ensure target
	if target == "" {
		target = "esp32"
	}

	// calculate paths
	espTool := filepath.Join(IDFDirectory(naosPath), "components", "esptool_py", "esptool", "esptool.py")
	if alt {
		var err error
		espTool, err = exec.LookPath("esptool.py")
		if err != nil {
			return err
		}
	}
	bootLoaderBinary := filepath.Join(Directory(naosPath), "build", "bootloader", "bootloader.bin")
	projectBinary := AppBinary(naosPath, appName)
	partitionsBinary := filepath.Join(Directory(naosPath), "build", "partition_table", "partition-table.bin")

	// prepare erase flash command
	eraseFlash := []string{
		espTool,
		"--port", port,
		"--baud", baudRate,
		"--before", "default_reset",
		"--after", "hard_reset",
		"erase_flash",
	}

	// prepare erase ota command
	eraseOTA := []string{
		espTool,
		"--port", port,
		"--baud", baudRate,
		"--before", "default_reset",
		"--after", "hard_reset",
		"erase_region", "0xd000", "0x2000",
	}

	// prepare boot loader offset
	bootLoader := "0x1000"
	if target == "esp32s3" {
		bootLoader = "0x0"
	}

	// TODO: Can we increase flash frequency and mode for some chips?

	// prepare flash all command
	flashAll := []string{
		espTool,
		"--port", port,
		"--baud", baudRate,
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash",
		"-z",
		"--flash_mode", "dio",
		"--flash_freq", "40m",
		"--flash_size", "detect",
		bootLoader, bootLoaderBinary,
		"0x8000", partitionsBinary,
		"0x10000", projectBinary,
	}

	// prepare flash app command
	flashApp := []string{
		espTool,
		"--port", port,
		"--baud", baudRate,
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
		err := Exec(naosPath, out, nil, alt, false, "python", eraseFlash...)
		if err != nil {
			return err
		}
	}

	// flash app only
	if appOnly {
		utils.Log(out, "Flashing (app only)...")
		err := Exec(naosPath, out, nil, alt, false, "python", flashApp...)
		if err != nil {
			return err
		}

		return nil
	}

	// flash all
	utils.Log(out, "Flashing...")
	err := Exec(naosPath, out, nil, alt, false, "python", flashAll...)
	if err != nil {
		return err
	}

	// erase ota if not already erased
	if !erase {
		utils.Log(out, "Erasing OTA config...")
		err := Exec(naosPath, out, nil, alt, false, "python", eraseOTA...)
		if err != nil {
			return err
		}
	}

	return nil
}
