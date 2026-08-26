package tree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
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
	// read flasher arguments, as the flash settings depend on the target
	args, err := readFlasherArgs(naosPath)
	if err != nil {
		return err
	}

	// read partition table, as the NVS partition may be placed anywhere
	partitions, err := readPartitions(naosPath)
	if err != nil {
		return err
	}

	// find NVS partition
	var nvsPart *partition
	for i, p := range partitions {
		if p.Name == "nvs" {
			nvsPart = &partitions[i]
			break
		}
	}
	if nvsPart == nil {
		return fmt.Errorf("missing 'nvs' partition")
	}

	// assemble CSV, sorting the keys to get a stable order
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteString("key,type,encoding,value\n")
	buf.WriteString("naos,namespace,,\n")
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("%s,data,binary,%s\n", key, values[key]))
	}

	// calculate paths
	tempDir := filepath.Join(os.TempDir(), "naos")
	valuesCSV := filepath.Join(tempDir, "values.csv")
	nvsImage := filepath.Join(tempDir, "nvs.bin")
	nvsPartGen := filepath.Join(IDFDirectory(naosPath), "components", "nvs_flash", "nvs_partition_generator", "nvs_partition_gen.py")
	espTool := filepath.Join(IDFDirectory(naosPath), "components", "esptool_py", "esptool", "esptool.py")

	// ensure directory
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		return err
	}

	// writing CSV
	utils.Log(out, "Writing values...")
	err = os.WriteFile(valuesCSV, buf.Bytes(), 0644)
	if err != nil {
		return err
	}

	// prepare arguments
	nvsPartGenArgs := []string{
		nvsPartGen,
		"generate",
		valuesCSV,
		nvsImage,
		fmt.Sprintf("0x%x", nvsPart.Size),
	}

	// generating image
	utils.Log(out, "Generating image...")
	err = Exec(naosPath, out, nil, false, false, "python", nvsPartGenArgs...)
	if err != nil {
		return err
	}

	// prepare flash arguments
	flashArgs := []string{espTool}
	if args.Extra.Chip != "" {
		flashArgs = append(flashArgs, "--chip", args.Extra.Chip)
	}
	flashArgs = append(flashArgs, "--port", port, "--baud", baudRate)
	flashArgs = append(flashArgs, "--before", orDefault(args.Extra.Before, "default_reset"))
	flashArgs = append(flashArgs, "--after", orDefault(args.Extra.After, "hard_reset"))
	flashArgs = append(flashArgs, "write_flash", "-z")
	flashArgs = append(flashArgs, args.WriteFlashArgs...)
	flashArgs = append(flashArgs, fmt.Sprintf("0x%x", nvsPart.Offset), nvsImage)

	// flashing image
	utils.Log(out, "Flashing...")
	err = Exec(naosPath, out, nil, false, false, "python", flashArgs...)
	if err != nil {
		return err
	}

	return nil
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
	var nvsPart *partition
	for i, p := range partitions {
		if p.Name == "nvs" {
			nvsPart = &partitions[i]
			break
		}
	}
	if nvsPart == nil {
		return nil, fmt.Errorf("missing 'nvs' partition")
	}

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

	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := espflasher.New(port, opts)
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

// typeBlobData is the entry type ESP-IDF uses for the chunks of a stored blob.
// The NVS parser does not decode chunked blobs and yields their index and data
// entries as raw entries instead.
const typeBlobData = 0x42

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
