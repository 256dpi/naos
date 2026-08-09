package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a trimmed down copy of the file written by an ESP-IDF build, note that the
// bootloader offset differs per target
const flasherArgsJSON = `{
    "write_flash_args" : [ "--flash_mode", "dio",
                           "--flash_size", "4MB",
                           "--flash_freq", "80m" ],
    "flash_settings" : {
        "flash_mode": "dio",
        "flash_size": "4MB",
        "flash_freq": "80m"
    },
    "flash_files" : {
        "0x0" : "bootloader/bootloader.bin",
        "0x8000" : "partition_table/partition-table.bin",
        "0xd000" : "ota_data_initial.bin",
        "0x10000" : "my-device.bin"
    },
    "bootloader" : { "offset" : "0x0", "file" : "bootloader/bootloader.bin", "encrypted" : "false" },
    "partition-table" : { "offset" : "0x8000", "file" : "partition_table/partition-table.bin", "encrypted" : "false" },
    "otadata" : { "offset" : "0xd000", "file" : "ota_data_initial.bin", "encrypted" : "false" },
    "app" : { "offset" : "0x10000", "file" : "my-device.bin", "encrypted" : "false" },
    "extra_esptool_args" : {
        "after"  : "hard_reset",
        "before" : "default_reset",
        "stub"   : true,
        "chip"   : "esp32c6"
    }
}`

func prepareFlasherArgs(t *testing.T) (string, *flasherArgs) {
	t.Helper()

	// decode arguments
	var args flasherArgs
	err := json.Unmarshal([]byte(flasherArgsJSON), &args)
	if err != nil {
		t.Fatal(err)
	}

	// prepare a build directory with an otadata image
	buildDir := t.TempDir()
	err = os.WriteFile(filepath.Join(buildDir, "ota_data_initial.bin"), make([]byte, 0x2000), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return buildDir, &args
}

func TestFlashStepsUsesFlasherArgs(t *testing.T) {
	buildDir, args := prepareFlasherArgs(t)

	steps, err := flashSteps(buildDir, "esptool.py", "/dev/tty", "921600", args, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected two steps, got %d", len(steps))
	}

	// the write must use the target specific offsets and the configured flash
	// settings instead of hard-coded values
	flash := strings.Join(steps[0].args, " ")
	expected := "esptool.py --chip esp32c6 --port /dev/tty --baud 921600 --before default_reset --after hard_reset " +
		"write_flash -z --flash_mode dio --flash_size 4MB --flash_freq 80m " +
		"0x0 " + filepath.Join(buildDir, "bootloader/bootloader.bin") + " " +
		"0x8000 " + filepath.Join(buildDir, "partition_table/partition-table.bin") + " " +
		"0x10000 " + filepath.Join(buildDir, "my-device.bin")
	if flash != expected {
		t.Fatalf("unexpected flash command:\n got: %s\nwant: %s", flash, expected)
	}

	// the ota config must be erased using the offset and size of the partition
	erase := strings.Join(steps[1].args, " ")
	if !strings.HasSuffix(erase, "erase_region 0xd000 0x2000") {
		t.Fatalf("unexpected erase command: %s", erase)
	}
}

func TestFlashStepsEraseAndAppOnly(t *testing.T) {
	buildDir, args := prepareFlasherArgs(t)

	// a full erase makes the separate ota erase obsolete
	steps, err := flashSteps(buildDir, "esptool.py", "/dev/tty", "921600", args, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected two steps, got %d", len(steps))
	}
	if !strings.HasSuffix(strings.Join(steps[0].args, " "), "erase_flash") {
		t.Fatalf("expected an erase, got %v", steps[0].args)
	}

	// an app only flash writes just the application
	steps, err = flashSteps(buildDir, "esptool.py", "/dev/tty", "921600", args, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
	app := strings.Join(steps[0].args, " ")
	if !strings.HasSuffix(app, "0x10000 "+filepath.Join(buildDir, "my-device.bin")) {
		t.Fatalf("unexpected app command: %s", app)
	}
	if strings.Contains(app, "bootloader") {
		t.Fatalf("expected no bootloader, got: %s", app)
	}
}

func TestFlashStepsWithoutOTAData(t *testing.T) {
	buildDir, args := prepareFlasherArgs(t)

	// a factory only layout has no ota data to erase
	args.OTAData = flasherArgsItem{}
	steps, err := flashSteps(buildDir, "esptool.py", "/dev/tty", "921600", args, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
}
