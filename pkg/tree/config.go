package tree

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
	var nvs *partition
	for i, p := range partitions {
		if p.Name == "nvs" {
			nvs = &partitions[i]
			break
		}
	}
	if nvs == nil {
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
		fmt.Sprintf("0x%x", nvs.Size),
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
	flashArgs = append(flashArgs, fmt.Sprintf("0x%x", nvs.Offset), nvsImage)

	// flashing image
	utils.Log(out, "Flashing...")
	err = Exec(naosPath, out, nil, false, false, "python", flashArgs...)
	if err != nil {
		return err
	}

	return nil
}
