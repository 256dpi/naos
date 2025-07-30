package tree

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

type bundleManifest struct {
	Target    string         `json:"target"`
	FlashMode string         `json:"flashMode"`
	FlashSize string         `json:"flashSize"`
	FlashFreq string         `json:"flashFreq"`
	Regions   []bundleRegion `json:"regions"`
}

type bundleRegion struct {
	Name   string `json:"name"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	File   string `json:"file,omitempty"`
	Fill   uint8  `json:"fill,omitempty"`
}

type flasherArgs struct {
	FlashSettings struct {
		FlashMode string `json:"flash_mode"`
		FlashSize string `json:"flash_size"`
		FlashFreq string `json:"flash_freq"`
	} `json:"flash_settings"`
	Bootloader       flasherArgsItem `json:"bootloader"`
	App              flasherArgsItem `json:"app"`
	PartitionTable   flasherArgsItem `json:"partition-table"`
	OtaData          flasherArgsItem `json:"otadata"`
	ExtraEsptoolArgs struct {
		Chip string `json:"chip"`
	} `json:"extra_esptool_args"`
}

type flasherArgsItem struct {
	Offset string `json:"offset"`
	File   string `json:"file"`
}

func Bundle(naosPath, appName, file string, out io.Writer) error {
	// ensure file name
	if file == "" {
		file = "bundle.zip"
	}

	// read flasher arguments
	data, err := os.ReadFile(filepath.Join(Directory(naosPath), "build", "flasher_args.json"))
	if err != nil {
		return fmt.Errorf("failed to read flasher arguments: %w", err)
	}
	var args flasherArgs
	err = json.Unmarshal(data, &args)
	if err != nil {
		return fmt.Errorf("failed to decode flasher arguments: %w", err)
	}

	// prepare binary paths
	bootLoaderBinary := filepath.Join(Directory(naosPath), "build", "bootloader", "bootloader.bin")
	partitionsBinary := filepath.Join(Directory(naosPath), "build", "partition_table", "partition-table.bin")
	otaDataBinary := filepath.Join(Directory(naosPath), "build", "ota_data_initial.bin")
	projectBinary := AppBinary(naosPath, appName)

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

	// prepare manifest
	manifest := bundleManifest{
		Target:    args.ExtraEsptoolArgs.Chip,
		FlashMode: args.FlashSettings.FlashMode,
		FlashSize: args.FlashSettings.FlashSize,
		FlashFreq: args.FlashSettings.FlashFreq,
		Regions: []bundleRegion{
			{
				Name:   "bootloader",
				Offset: mustParseHex(args.Bootloader.Offset),
				Size:   bootLoaderStat.Size(),
				File:   filepath.Base(bootLoaderBinary),
			},
			{
				Name:   "partition-table",
				Offset: mustParseHex(args.PartitionTable.Offset),
				Size:   partitionsStat.Size(),
				File:   filepath.Base(partitionsBinary),
			},
			{
				Name:   "application",
				Offset: mustParseHex(args.App.Offset),
				Size:   projectStat.Size(),
				File:   filepath.Base(projectBinary),
			},
		},
	}

	// add OTA region if available
	if args.OtaData.Offset != "" {
		stat, err := os.Stat(otaDataBinary)
		if err != nil {
			return fmt.Errorf("failed to stat OTA data binary: %w", err)
		}
		manifest.Regions = append(manifest.Regions, bundleRegion{
			Name:   "ota-data",
			Offset: mustParseHex(args.OtaData.Offset),
			Size:   stat.Size(),
			Fill:   0xFF,
		})
	}

	// create archive
	utils.Log(out, fmt.Sprintf("Creating %s...", file))
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	// create writer
	w := zip.NewWriter(f)
	defer w.Close()

	// write binary files
	for _, binary := range []string{
		bootLoaderBinary,
		partitionsBinary,
		projectBinary,
	} {
		utils.Log(out, fmt.Sprintf("Writing %s...", filepath.Base(binary)))
		data, err := os.ReadFile(binary)
		if err != nil {
			return err
		}
		zw, err := w.Create(filepath.Base(binary))
		if err != nil {
			return err
		}
		_, err = zw.Write(data)
		if err != nil {
			return err
		}
	}

	// write manifest
	utils.Log(out, "Writing manifest.json...")
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

	return nil
}

func mustParseHex(s string) int64 {
	var value int64
	_, err := fmt.Sscanf(s, "0x%x", &value)
	if err != nil {
		panic(fmt.Sprintf("failed to parse hex value %s: %v", s, err))
	}
	return value
}
