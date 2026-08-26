package tree

import (
	"bytes"
	"encoding/binary"
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

// coredumpHeaderSize is the size of the header stored in front of a coredump,
// which begins with the total length of the dump.
const coredumpHeaderSize = 16

// coredumpLength will determine the length of the stored coredump. An implausible
// length is answered with the size of the partition, the same way espcoredump
// handles it, and left to the parser to reject.
func coredumpLength(header []byte, size int64) (uint32, error) {
	// get stored length
	length := binary.LittleEndian.Uint32(header[0:4])

	// check if partition is erased
	if length == 0xffffffff {
		return 0, fmt.Errorf("no coredump stored on device")
	}

	// fall back to the partition size
	if length == 0 || int64(length) > size {
		return uint32(size), nil
	}

	return length, nil
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

	// reset device on return, to leave it running the application
	defer flasher.Reset()

	// find coredump partition
	part, err := findDevicePartition(flasher, naosPath, "coredump")
	if err != nil {
		return nil, err
	}

	// read header, which carries the length of the stored dump
	header, err := flasher.ReadFlash(uint32(part.Offset), coredumpHeaderSize, nil)
	if err != nil {
		return nil, err
	}

	// determine length
	length, err := coredumpLength(header, part.Size)
	if err != nil {
		return nil, err
	}

	// read dump
	utils.Log(out, "Reading...")
	data, err := flasher.ReadFlash(uint32(part.Offset), length, progressLogger(out))
	if err != nil {
		return nil, err
	}

	return ParseCoredump(naosPath, appName, elf, data)
}
