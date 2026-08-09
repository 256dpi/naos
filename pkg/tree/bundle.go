package tree

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

type bundleManifest struct {
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	Target    string         `json:"target"`
	FlashMode string         `json:"flashMode"`
	FlashSize string         `json:"flashSize"`
	FlashFreq string         `json:"flashFreq"`
	Regions   []bundleRegion `json:"regions"`
	DebugFile string         `json:"debugFile,omitempty"`
}

type bundleRegion struct {
	Name   string `json:"name"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	File   string `json:"file,omitempty"`
	Fill   uint8  `json:"fill,omitempty"`
}

type projectDescription struct {
	Name    string `json:"project_name"`
	Version string `json:"project_version"`
	Target  string `json:"target"`
}

type flasherArgs struct {
	WriteFlashArgs []string `json:"write_flash_args"`
	Flash          struct {
		Mode string `json:"flash_mode"`
		Size string `json:"flash_size"`
		Freq string `json:"flash_freq"`
	} `json:"flash_settings"`
	Bootloader     flasherArgsItem `json:"bootloader"`
	Application    flasherArgsItem `json:"app"`
	PartitionTable flasherArgsItem `json:"partition-table"`
	OTAData        flasherArgsItem `json:"otadata"`
	Extra          struct {
		Before string `json:"before"`
		After  string `json:"after"`
		Chip   string `json:"chip"`
	} `json:"extra_esptool_args"`
}

type flasherArgsItem struct {
	Offset string `json:"offset"`
	File   string `json:"file"`
}

// readFlasherArgs will read the flasher arguments written by the build. They
// carry the target specific offsets and flash settings and are therefore
// preferred over hard-coded values.
func readFlasherArgs(naosPath string) (*flasherArgs, error) {
	// read flasher arguments
	data, err := os.ReadFile(filepath.Join(Directory(naosPath), "build", "flasher_args.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read flasher arguments: %w", err)
	}

	// decode flasher arguments
	var args flasherArgs
	err = json.Unmarshal(data, &args)
	if err != nil {
		return nil, fmt.Errorf("failed to decode flasher arguments: %w", err)
	}

	return &args, nil
}

func Bundle(naosPath, file string, addDebug bool, out io.Writer) error {
	// read project description
	descFile := filepath.Join(Directory(naosPath), "build", "project_description.json")
	data, err := os.ReadFile(descFile)
	if err != nil {
		return fmt.Errorf("failed to read project description: %w", err)
	}
	var desc projectDescription
	err = json.Unmarshal(data, &desc)
	if err != nil {
		return fmt.Errorf("failed to decode project description: %w", err)
	}

	// read flasher arguments
	args, err := readFlasherArgs(naosPath)
	if err != nil {
		return err
	}

	// prepare binary paths
	bootLoaderBinary := filepath.Join(Directory(naosPath), "build", "bootloader", "bootloader.bin")
	partitionsBinary := filepath.Join(Directory(naosPath), "build", "partition_table", "partition-table.bin")
	otaDataBinary := filepath.Join(Directory(naosPath), "build", "ota_data_initial.bin")
	projectBinary := AppBinary(naosPath, desc.Name)
	projectELF := AppELF(naosPath, desc.Name)

	// get binary sizes
	bootLoaderStat, err := os.Stat(bootLoaderBinary)
	if err != nil {
		return fmt.Errorf("failed to stat bootloader binary: %w", err)
	}
	partitionsStat, err := os.Stat(partitionsBinary)
	if err != nil {
		return fmt.Errorf("failed to stat partition table binary: %w", err)
	}
	projectStat, err := os.Stat(projectBinary)
	if err != nil {
		return fmt.Errorf("failed to stat project binary: %w", err)
	}

	// parse offsets
	bootLoaderOffset, err := parseHex(args.Bootloader.Offset)
	if err != nil {
		return fmt.Errorf("failed to parse bootloader offset: %w", err)
	}
	partitionsOffset, err := parseHex(args.PartitionTable.Offset)
	if err != nil {
		return fmt.Errorf("failed to parse partition table offset: %w", err)
	}
	projectOffset, err := parseHex(args.Application.Offset)
	if err != nil {
		return fmt.Errorf("failed to parse application offset: %w", err)
	}

	// prepare manifest
	manifest := bundleManifest{
		Name:      desc.Name,
		Version:   desc.Version,
		Target:    desc.Target,
		FlashMode: args.Flash.Mode,
		FlashSize: args.Flash.Size,
		FlashFreq: args.Flash.Freq,
		Regions: []bundleRegion{
			{
				Name:   "bootloader",
				Offset: bootLoaderOffset,
				Size:   bootLoaderStat.Size(),
				File:   filepath.Base(bootLoaderBinary),
			},
			{
				Name:   "partition-table",
				Offset: partitionsOffset,
				Size:   partitionsStat.Size(),
				File:   filepath.Base(partitionsBinary),
			},
			{
				Name:   "application",
				Offset: projectOffset,
				Size:   projectStat.Size(),
				File:   filepath.Base(projectBinary),
			},
		},
	}

	// add debug file if requested
	if addDebug {
		manifest.DebugFile = filepath.Base(projectELF)
	}

	// add OTA region if available
	if args.OTAData.Offset != "" {
		stat, err := os.Stat(otaDataBinary)
		if err != nil {
			return fmt.Errorf("failed to stat OTA data binary: %w", err)
		}
		offset, err := parseHex(args.OTAData.Offset)
		if err != nil {
			return fmt.Errorf("failed to parse OTA data offset: %w", err)
		}
		manifest.Regions = append(manifest.Regions, bundleRegion{
			Name:   "ota-data",
			Offset: offset,
			Size:   stat.Size(),
			Fill:   0xFF,
		})
	}

	// ensure file name
	if file == "" {
		file = manifest.Name + "-" + manifest.Version + ".zip"
	}

	// create archive
	utils.Log(out, fmt.Sprintf("Creating %s...", file))
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	// make sure the file gets closed on error, as both are closed explicitly
	// below to be able to report a failure to flush the archive
	defer f.Close()

	// create writer
	w := zip.NewWriter(f)

	// prepare files
	addFiles := []string{
		bootLoaderBinary,
		partitionsBinary,
		projectBinary,
	}
	if addDebug {
		addFiles = append(addFiles, projectELF)
	}

	// write files
	for _, addFile := range addFiles {
		data, err := os.ReadFile(addFile)
		if err != nil {
			return err
		}
		zw, err := w.Create(filepath.Base(addFile))
		if err != nil {
			return err
		}
		_, err = zw.Write(data)
		if err != nil {
			return err
		}
	}

	// write manifest
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	zw, err := w.Create("manifest.json")
	if err != nil {
		return err
	}
	_, err = zw.Write(manifestData)
	if err != nil {
		return err
	}

	// flush archive
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to write archive: %w", err)
	}

	// properly close file
	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close archive: %w", err)
	}

	// print manifest
	_, _ = fmt.Fprintln(out, string(manifestData))

	return nil
}

// parseHex will parse a hexadecimal offset as written by the build.
func parseHex(s string) (int64, error) {
	// parse value
	value, err := strconv.ParseInt(strings.TrimPrefix(s, "0x"), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed hex value %q", s)
	}

	return value, nil
}
