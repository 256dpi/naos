package tree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"

	"tinygo.org/x/espflasher/pkg/espflasher"

	"github.com/256dpi/naos/pkg/utils"
)

// flasherLogger adapts the tree logging to the flasher logger interface.
type flasherLogger struct {
	out io.Writer
}

// Logf implements the espflasher.Logger interface.
func (l flasherLogger) Logf(format string, args ...interface{}) {
	utils.Log(l.out, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

// progressLogger returns a progress function that logs the progress of a long
// running operation in steps of ten percent.
func progressLogger(out io.Writer) espflasher.ProgressFunc {
	var last int
	return func(current, total int) {
		// check total
		if total <= 0 {
			return
		}

		// determine step, skipping steps that have been logged already
		step := min(current*100/total, 100) / 10 * 10
		if step <= last {
			return
		}
		last = step

		// log step
		utils.Log(out, fmt.Sprintf("%d%%...", step))
	}
}

// connectDevice will connect to the device on the specified serial port. If
// flasher arguments are provided, their flash settings are applied to written
// images, the same way esptool patches the image header.
func connectDevice(port, baudRate string, args *flasherArgs, out io.Writer) (*espflasher.Flasher, error) {
	// parse baud rate
	rate, err := strconv.Atoi(baudRate)
	if err != nil {
		return nil, fmt.Errorf("invalid baud rate: %w", err)
	}

	// prepare options, using the requested rate for the transfer only, as the
	// initial synchronization is more reliable at the default rate
	opts := espflasher.DefaultOptions()
	opts.FlashBaudRate = rate
	opts.Logger = flasherLogger{out: out}

	// apply flash settings
	if args != nil {
		opts.FlashMode = args.Flash.Mode
		opts.FlashSize = args.Flash.Size
		opts.FlashFreq = args.Flash.Freq
	}

	return espflasher.New(port, opts)
}

// partition describes an entry of the partition table.
type partition struct {
	Name   string
	Offset int64
	Size   int64
}

// The magic and size of a partition table entry as well as the maximum length
// and default offset of the table. The layout is fixed, as the bootloader
// itself parses the table, and has not changed since it has been introduced.
const (
	partitionMagic      = 0x50aa
	partitionEntrySize  = 32
	partitionTableSize  = 0xc00
	partitionTableStart = 0x8000
)

// readPartitions will read and parse the partition table from the device. The
// table is read from the device, rather than taken from the build, as a device
// may have been flashed with a different layout.
func readPartitions(flasher *espflasher.Flasher, offset uint32) ([]partition, error) {
	// read table
	data, err := flasher.ReadFlash(offset, partitionTableSize, nil)
	if err != nil {
		return nil, err
	}

	// parse table
	partitions := parsePartitions(data)
	if len(partitions) == 0 {
		return nil, fmt.Errorf("no partition table found at 0x%x", offset)
	}

	return partitions, nil
}

// parsePartitions will parse the entries of a partition table
// ("magic, type, subtype, offset, size, label, flags").
func parsePartitions(data []byte) []partition {
	var partitions []partition
	for len(data) >= partitionEntrySize {
		// get entry
		entry := data[:partitionEntrySize]
		data = data[partitionEntrySize:]

		// stop at the first other entry, which is either the appended MD5 sum
		// or the erased remainder of the table
		if binary.LittleEndian.Uint16(entry[0:2]) != partitionMagic {
			break
		}

		// collect partition
		partitions = append(partitions, partition{
			Name:   string(bytes.TrimRight(entry[12:28], "\x00")),
			Offset: int64(binary.LittleEndian.Uint32(entry[4:8])),
			Size:   int64(binary.LittleEndian.Uint32(entry[8:12])),
		})
	}

	return partitions
}

// findPartition will return the named partition.
func findPartition(partitions []partition, name string) (*partition, error) {
	for i, p := range partitions {
		if p.Name == name {
			return &partitions[i], nil
		}
	}

	return nil, fmt.Errorf("missing %q partition", name)
}

// findDevicePartition will read the partition table from the device and return
// the named partition.
func findDevicePartition(flasher *espflasher.Flasher, naosPath, name string) (*partition, error) {
	// read partition table
	partitions, err := readPartitions(flasher, partitionTableOffset(naosPath))
	if err != nil {
		return nil, err
	}

	return findPartition(partitions, name)
}

// partitionTableOffset will return the offset at which the partition table is
// stored, as configured by the build, falling back to the default offset.
func partitionTableOffset(naosPath string) uint32 {
	// read flasher arguments, which are missing if the project has not been
	// built yet
	args, err := readFlasherArgs(naosPath)
	if err != nil {
		return partitionTableStart
	}

	// parse offset
	offset, err := parseOffset(args.PartitionTable.Offset)
	if err != nil {
		return partitionTableStart
	}

	return offset
}
