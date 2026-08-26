package tree

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// ParseCoredump will parse the provided raw coredump data and return a
// human-readable representation.
func ParseCoredump(naosPath, appName, elf string, coredump []byte) ([]byte, error) {
	// get paths
	espCoredump := filepath.Join(IDFDirectory(naosPath), "components", "espcoredump", "espcoredump.py")

	// get a temporary file
	file, err := os.CreateTemp("", "coredump")
	if err != nil {
		return nil, err
	}

	// ensure file gets closed
	defer file.Close()

	// write core dump to file
	_, err = file.Write(coredump)
	if err != nil {
		return nil, err
	}

	// close file
	err = file.Close()
	if err != nil {
		return nil, err
	}

	// ensure ELF
	if elf == "" {
		elf = AppELF(naosPath, appName)
	}

	// create buffer
	buf := new(bytes.Buffer)

	// parse coredump
	err = Exec(naosPath, buf, nil, false, false, espCoredump, "info_corefile", "-t", "raw", "-c", file.Name(), elf)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, buf.String())
	}

	// delete file
	err = utils.Remove(file.Name())
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// LoadCoredump will read the coredump from the flash of an attached device and
// return a human-readable representation.
func LoadCoredump(naosPath, appName, elf, port, baudRate string, out io.Writer) ([]byte, error) {
	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := connectDevice(port, baudRate, nil, out)
	if err != nil {
		return nil, err
	}
	defer flasher.Close()

	// find coredump partition
	part, err := findDevicePartition(flasher, naosPath, "coredump")
	if err != nil {
		return nil, err
	}

	// read partition
	utils.Log(out, "Reading...")
	data, err := flasher.ReadFlash(uint32(part.Offset), uint32(part.Size), progressLogger(out))
	if err != nil {
		return nil, err
	}

	// reset device to leave it running the application
	flasher.Reset()

	return ParseCoredump(naosPath, appName, elf, data)
}
