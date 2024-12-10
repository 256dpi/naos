package tree

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// Config will write settings and parameters to an attached device.
func Config(naosPath string, values map[string]string, port, baudRate string, out io.Writer) error {
	// assemble CSV
	var buf bytes.Buffer
	buf.WriteString("key,type,encoding,value\n")
	buf.WriteString("naos,namespace,,\n")
	for key, value := range values {
		buf.WriteString(fmt.Sprintf("%s,data,string,%s\n", key, value))
	}

	// calculate paths
	tempDir := filepath.Join(os.TempDir(), "naos")
	valuesCSV := filepath.Join(tempDir, "values.csv")
	nvsImage := filepath.Join(tempDir, "nvs.bin")
	nvsPartGen := filepath.Join(IDFDirectory(naosPath), "components", "nvs_flash", "nvs_partition_generator", "nvs_partition_gen.py")
	espTool := filepath.Join(IDFDirectory(naosPath), "components", "esptool_py", "esptool", "esptool.py")

	// ensure directory
	err := os.MkdirAll(tempDir, 0755)
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
		"0x4000",
	}

	// generating image
	utils.Log(out, "Generating image...")
	err = Exec(naosPath, out, nil, false, false, "python", nvsPartGenArgs...)
	if err != nil {
		return err
	}

	// flashing image
	utils.Log(out, "Flashing...")
	err = Exec(naosPath, out, nil, false, false, "python", []string{
		espTool,
		"--port", port,
		"--baud", baudRate,
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash",
		"-z",
		"--flash_mode", "dio",
		"--flash_freq", "40m",
		"--flash_size", "detect",
		"0x9000", nvsImage,
	}...)
	if err != nil {
		return err
	}

	return nil
}
