package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

	steps, err := flashSteps(buildDir, args, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected two steps, got %d", len(steps))
	}

	// the write must use the target specific offsets instead of hard-coded ones
	want := []flashImage{
		{offset: 0x0, file: filepath.Join(buildDir, "bootloader/bootloader.bin")},
		{offset: 0x8000, file: filepath.Join(buildDir, "partition_table/partition-table.bin")},
		{offset: 0x10000, file: filepath.Join(buildDir, "my-device.bin")},
	}
	if steps[0].kind != flashWrite || !reflect.DeepEqual(steps[0].images, want) {
		t.Fatalf("unexpected write step: %+v", steps[0])
	}

	// the ota config must be erased using the offset and size of the partition
	if steps[1].kind != flashEraseRegion || steps[1].offset != 0xd000 || steps[1].size != 0x2000 {
		t.Fatalf("unexpected erase step: %+v", steps[1])
	}
}

func TestESPToolArgs(t *testing.T) {
	buildDir, args := prepareFlasherArgs(t)

	steps, err := flashSteps(buildDir, args, false, false)
	if err != nil {
		t.Fatal(err)
	}

	// the write must use the target specific offsets and the configured flash
	// settings instead of hard-coded values
	flash := strings.Join(espToolArgs(steps[0], "/dev/tty", "921600", args), " ")
	expected := "--chip esp32c6 --port /dev/tty --baud 921600 --before default_reset --after hard_reset " +
		"write_flash -z --flash_mode dio --flash_size 4MB --flash_freq 80m " +
		"0x0 " + filepath.Join(buildDir, "bootloader/bootloader.bin") + " " +
		"0x8000 " + filepath.Join(buildDir, "partition_table/partition-table.bin") + " " +
		"0x10000 " + filepath.Join(buildDir, "my-device.bin")
	if flash != expected {
		t.Fatalf("unexpected flash command:\n got: %s\nwant: %s", flash, expected)
	}

	// the ota config must be erased using the offset and size of the partition
	erase := strings.Join(espToolArgs(steps[1], "/dev/tty", "921600", args), " ")
	if !strings.HasSuffix(erase, "erase_region 0xd000 0x2000") {
		t.Fatalf("unexpected erase command: %s", erase)
	}
}

func TestFlashStepsEraseAndAppOnly(t *testing.T) {
	buildDir, args := prepareFlasherArgs(t)

	// a full erase makes the separate ota erase obsolete
	steps, err := flashSteps(buildDir, args, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected two steps, got %d", len(steps))
	}
	if steps[0].kind != flashErase {
		t.Fatalf("expected an erase, got %+v", steps[0])
	}

	// an app only flash writes just the application
	steps, err = flashSteps(buildDir, args, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
	want := []flashImage{{offset: 0x10000, file: filepath.Join(buildDir, "my-device.bin")}}
	if steps[0].kind != flashWrite || !reflect.DeepEqual(steps[0].images, want) {
		t.Fatalf("unexpected app step: %+v", steps[0])
	}
}

func TestFlashStepsWithoutOTAData(t *testing.T) {
	buildDir, args := prepareFlasherArgs(t)

	// a factory only layout has no ota data to erase
	args.OTAData = flasherArgsItem{}
	steps, err := flashSteps(buildDir, args, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
}

func TestVerifyChip(t *testing.T) {
	for _, item := range []struct {
		detected string
		chip     string
		fail     bool
	}{
		{"ESP32-S3", "esp32s3", false},
		{"ESP32", "esp32", false},
		{"ESP32-P4-Rev1", "esp32p4", false},
		{"ESP8266", "esp8266", false},
		{"ESP32-S3", "", false},
		{"ESP32-C6", "esp32s3", true},
		{"ESP32", "esp32s3", true},
	} {
		err := verifyChip(item.detected, item.chip)
		if (err != nil) != item.fail {
			t.Errorf("%s/%s: unexpected result: %v", item.detected, item.chip, err)
		}
	}
}
