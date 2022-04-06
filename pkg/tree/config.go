package tree

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

var settings = map[string]bool{
	"wifi-ssid":      true,
	"wifi-password":  true,
	"mqtt-host":      true,
	"mqtt-port":      true,
	"mqtt-username":  true,
	"mqtt-password":  true,
	"mqtt-client-id": true,
	"device-name":    true,
	"base-topic":     true,
}

// Config will write settings and parameters to an attached device.
func Config(naosPath string, values map[string]string, port string, out io.Writer) error {
	// get IDF major version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// assemble csv
	var buf bytes.Buffer
	buf.WriteString("key,type,encoding,value\n")
	buf.WriteString("naos-ble,namespace,,\n")
	for key, value := range values {
		if settings[key] {
			buf.WriteString(fmt.Sprintf("%s,data,string,%s\n", key, value))
		}
	}
	buf.WriteString("naos-manager,namespace,,\n")
	for key, value := range values {
		if !settings[key] {
			buf.WriteString(fmt.Sprintf("%s,data,string,%s\n", key, value))
		}
	}

	// calculate paths
	tempDir := filepath.Join(os.TempDir(), "naos")
	valuesCSV := filepath.Join(tempDir, "values.csv")
	nvsImage := filepath.Join(tempDir, "nvs.img")
	if idfMajorVersion == 4 {
		nvsImage = filepath.Join(tempDir, "nvs.bin")
	}
	nvsPartGen := filepath.Join(IDFDirectory(naosPath), "components", "nvs_flash", "nvs_partition_generator", "nvs_partition_gen.py")
	espTool := filepath.Join(IDFDirectory(naosPath), "components", "esptool_py", "esptool", "esptool.py")

	// ensure directory
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		return err
	}

	// writing CSV
	utils.Log(out, "Writing values...")
	err = ioutil.WriteFile(valuesCSV, buf.Bytes(), 0644)
	if err != nil {
		return err
	}

	// prepare arguments
	var nvsPartGenArgs []string
	if idfMajorVersion == 3 {
		nvsPartGenArgs = []string{
			nvsPartGen,
			"--input", valuesCSV,
			"--output", nvsImage,
			"--size", "0x4000",
		}
	} else if idfMajorVersion == 4 {
		nvsPartGenArgs = []string{
			nvsPartGen,
			"generate",
			valuesCSV,
			nvsImage,
			"0x4000",
		}
	}

	// generating image
	utils.Log(out, "Generating image...")
	err = Exec(naosPath, out, nil, "python", nvsPartGenArgs...)
	if err != nil {
		return err
	}

	// flashing image
	utils.Log(out, "Flashing...")
	err = Exec(naosPath, out, nil, "python", []string{
		espTool,
		"--chip", "esp32",
		"--port", port,
		"--baud", "921600",
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
