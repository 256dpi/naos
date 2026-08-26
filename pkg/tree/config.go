package tree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"tinygo.org/x/espflasher/pkg/espflasher"
	"tinygo.org/x/espflasher/pkg/nvs"

	"github.com/256dpi/naos/pkg/utils"
)

// partition describes an entry of the built partition table.
type partition struct {
	Name   string
	Type   string
	Offset int64
	Size   int64
}

// readPartitions will return the partitions of the built partition table. The
// table is decoded using the ESP-IDF generator, as the offsets and sizes in the
// CSV may be left blank and are only resolved during the build.
func readPartitions(naosPath string) ([]partition, error) {
	// get paths
	generator := filepath.Join(IDFDirectory(naosPath), "components", "partition_table", "gen_esp32part.py")
	table := filepath.Join(Directory(naosPath), "build", "partition_table", "partition-table.bin")

	// decode table into a buffer, which also swallows the echoed command as it
	// does not parse as a table row
	buf := new(bytes.Buffer)
	err := Exec(naosPath, buf, nil, false, false, "python", generator, table, "-")
	if err != nil {
		return nil, fmt.Errorf("failed to read partition table: %w: %s", err, buf.String())
	}

	// parse table ("name,type,subtype,offset,size,flags")
	var partitions []partition
	for _, line := range strings.Split(buf.String(), "\n") {
		// skip comments and empty lines
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// split row
		row := strings.Split(line, ",")
		if len(row) < 5 {
			continue
		}

		// parse offset and size
		offset, err := parseSize(row[3])
		if err != nil {
			continue
		}
		size, err := parseSize(row[4])
		if err != nil {
			continue
		}

		// collect partition
		partitions = append(partitions, partition{
			Name:   row[0],
			Type:   row[1],
			Offset: offset,
			Size:   size,
		})
	}

	return partitions, nil
}

// parseSize will parse an offset or size as printed by the ESP-IDF partition
// table generator, which uses hexadecimal values and "K"/"M" suffixes.
func parseSize(str string) (int64, error) {
	// determine multiplier
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(str, "K"):
		multiplier, str = 1024, strings.TrimSuffix(str, "K")
	case strings.HasSuffix(str, "M"):
		multiplier, str = 1024*1024, strings.TrimSuffix(str, "M")
	}

	// parse hexadecimal values
	if rest, ok := strings.CutPrefix(str, "0x"); ok {
		value, err := strconv.ParseInt(rest, 16, 64)
		if err != nil {
			return 0, err
		}
		return value * multiplier, nil
	}

	// parse decimal values
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, err
	}

	return value * multiplier, nil
}

// Config will write settings and parameters to an attached device.
func Config(naosPath string, values map[string]string, port, baudRate string, out io.Writer) error {
	// read partition table, as the NVS partition may be placed anywhere
	partitions, err := readPartitions(naosPath)
	if err != nil {
		return err
	}

	// find NVS partition
	nvsPart, err := findPartition(partitions, "nvs")
	if err != nil {
		return err
	}

	// generate image
	utils.Log(out, "Generating image...")
	image, err := generateImage(values, int(nvsPart.Size))
	if err != nil {
		return err
	}

	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := connectDevice(port, baudRate, out)
	if err != nil {
		return err
	}
	defer flasher.Close()

	// flash image
	utils.Log(out, "Flashing...")
	err = flasher.FlashImage(image, uint32(nvsPart.Offset), nil)
	if err != nil {
		return err
	}

	// reset device to run the application with the new parameters
	flasher.Reset()

	return nil
}

// chunkMaxSize is the maximum size of a single blob chunk, which is the space
// left on a page beside the chunk's own entry.
const chunkMaxSize = nvs.EntrySize * (nvs.EntriesPerPage - 1)

// generateImage will generate an NVS partition image that stores the provided
// parameters.
func generateImage(values map[string]string, size int) ([]byte, error) {
	// assemble entries, sorting the keys to get a stable order
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var entries []nvs.Entry
	for _, key := range keys {
		// check size, as the chunk index and count are stored as a single byte
		if len(values[key]) > 255*chunkMaxSize {
			return nil, fmt.Errorf("value of %q is too big", key)
		}

		entries = append(entries, blobEntries(key, []byte(values[key]))...)
	}

	return nvs.GenerateNVS(entries, size)
}

// blobEntries will assemble the entries that store the provided value: one data
// entry per chunk and an index entry that describes the chunks. The generator
// only supports the single entry blobs of the earlier format, which are limited
// to one page, therefore the chunked entries written by ESP-IDF itself are
// assembled as raw entries.
func blobEntries(key string, value []byte) []nvs.Entry {
	// split value into chunks, storing empty values as a single empty chunk
	var chunks [][]byte
	for offset := 0; offset < len(value); offset += chunkMaxSize {
		chunks = append(chunks, value[offset:min(offset+chunkMaxSize, len(value))])
	}
	if len(chunks) == 0 {
		chunks = append(chunks, nil)
	}

	// prepare data entries
	entries := make([]nvs.Entry, 0, len(chunks)+1)
	for i, chunk := range chunks {
		// prepare data, padding the chunk to full entries with the erased state
		data := bytes.Repeat([]byte{0xff}, 8+alignSize(len(chunk)))
		copy(data[8:], chunk)

		// write header ("size, reserved, CRC")
		binary.LittleEndian.PutUint16(data[0:2], uint16(len(chunk)))
		binary.LittleEndian.PutUint32(data[4:8], espCRC32(chunk))

		// collect entry
		entries = append(entries, nvs.Entry{
			Namespace:  "naos",
			Key:        key,
			Raw:        true,
			TypeByte:   typeBlobData,
			Span:       uint8(1 + alignSize(len(chunk))/nvs.EntrySize),
			ChunkIndex: uint8(i),
			Data:       data,
		})
	}

	// prepare index ("size, chunk count, chunk start, reserved")
	index := bytes.Repeat([]byte{0xff}, 8)
	binary.LittleEndian.PutUint32(index[0:4], uint32(len(value)))
	index[4] = uint8(len(chunks))
	index[5] = 0

	// collect index entry
	entries = append(entries, nvs.Entry{
		Namespace:  "naos",
		Key:        key,
		Raw:        true,
		TypeByte:   typeBlobIndex,
		Span:       1,
		ChunkIndex: chunkIndexNone,
		Data:       index,
	})

	return entries
}

// alignSize will round the provided size up to full entries.
func alignSize(size int) int {
	return (size + nvs.EntrySize - 1) / nvs.EntrySize * nvs.EntrySize
}

// espCRC32 will calculate the CRC as used by NVS.
func espCRC32(data []byte) uint32 {
	return crc32.Update(0xffffffff, crc32.IEEETable, data)
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

// connectDevice will connect to the device on the specified serial port.
func connectDevice(port, baudRate string, out io.Writer) (*espflasher.Flasher, error) {
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

	return espflasher.New(port, opts)
}

// flasherLogger adapts the tree logging to the flasher logger interface.
type flasherLogger struct {
	out io.Writer
}

// Logf implements the espflasher.Logger interface.
func (l flasherLogger) Logf(format string, args ...interface{}) {
	utils.Log(l.out, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

// ReadConfig will read the parameters stored on an attached device.
func ReadConfig(naosPath, port, baudRate string, out io.Writer) (map[string]string, error) {
	// read partition table, as the NVS partition may be placed anywhere
	partitions, err := readPartitions(naosPath)
	if err != nil {
		return nil, err
	}

	// find NVS partition
	nvsPart, err := findPartition(partitions, "nvs")
	if err != nil {
		return nil, err
	}

	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := connectDevice(port, baudRate, out)
	if err != nil {
		return nil, err
	}
	defer flasher.Close()

	// read partition
	utils.Log(out, "Reading...")
	data, err := flasher.ReadFlash(uint32(nvsPart.Offset), uint32(nvsPart.Size), nil)
	if err != nil {
		return nil, err
	}

	// reset device to leave it running the application
	flasher.Reset()

	// parse partition
	entries, err := nvs.ParseNVS(data)
	if err != nil {
		return nil, err
	}

	return collectValues(entries, out), nil
}

// The entry types ESP-IDF uses to store a blob as a set of chunks, and the
// chunk index used by entries that are not a chunk. The NVS parser does not
// decode chunked blobs and yields their entries as raw entries instead.
const (
	typeBlobData   = 0x42
	typeBlobIndex  = 0x48
	chunkIndexNone = 0xff
)

// collectValues will extract the parameters stored in the "naos" namespace from
// the provided NVS entries. Values are stored as blobs, which ESP-IDF splits
// into chunks that must be reassembled in order.
func collectValues(entries []nvs.Entry, out io.Writer) map[string]string {
	// collect values and chunks
	values := map[string]string{}
	chunks := map[string]map[uint8][]byte{}
	for _, entry := range entries {
		// skip other namespaces
		if entry.Namespace != "naos" {
			continue
		}

		// handle legacy single entry blobs
		if !entry.Raw && entry.Type == "blob" {
			if value, ok := entry.Value.([]byte); ok {
				values[entry.Key] = string(value)
			}
			continue
		}

		// skip other entries, especially the blob index entries, which only
		// describe the chunks collected below
		if !entry.Raw || entry.TypeByte != typeBlobData {
			continue
		}

		// get size, which is stored in the first two bytes of the entry header
		if len(entry.Data) < 8 {
			continue
		}
		size := int(binary.LittleEndian.Uint16(entry.Data[0:2]))
		if 8+size > len(entry.Data) {
			continue
		}

		// store chunk
		if chunks[entry.Key] == nil {
			chunks[entry.Key] = map[uint8][]byte{}
		}
		chunks[entry.Key][entry.ChunkIndex] = entry.Data[8 : 8+size]
	}

	// assemble chunked values
	for key, parts := range chunks {
		// sort chunks
		indexes := make([]int, 0, len(parts))
		for index := range parts {
			indexes = append(indexes, int(index))
		}
		sort.Ints(indexes)

		// concatenate chunks
		var value []byte
		for _, index := range indexes {
			value = append(value, parts[uint8(index)]...)
		}

		values[key] = string(value)
	}

	// drop binary values, as they cannot be represented in the configuration
	for key, value := range values {
		if !utf8.ValidString(value) {
			utils.Log(out, fmt.Sprintf("Skipping binary value: %s", key))
			delete(values, key)
		}
	}

	return values
}
