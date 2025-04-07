package tree

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

// TODO: Changing embeds in a v4 projects requires a clean.
// TODO: Updating sdkconfig.overrides requires a reconfigure.
// TODO: Changing target needs a reconfigure and clean.

// Partitions defines a percentage based partitioning scheme.
type Partitions struct {
	Total   int // MiBs
	Alpha   int // %
	Beta    int // %
	Storage int // %
}

func (p *Partitions) generate() (string, error) {
	// check if values add up
	if p.Alpha+p.Beta+p.Storage != 100 {
		return "", fmt.Errorf("partitions do not add up to 100%%")
	}

	// prepare partitions
	partitions := `# Name,   Type, SubType,  Offset,  Size
nvs,      data, nvs,      0x9000,  0x4000
otadata,  data, ota,      0xd000,  0x2000
phy_init, data, phy,      0xf000,  0x1000
alpha,    app,  ota_0,    0x10000, ALPHA_BYTES
beta,     app,  ota_1,    ,        BETA_BYTES
storage,  data, fat,      ,        STORAGE_BYTES
coredump, data, coredump, ,        64K
`

	// calculate available bytes
	total := int64(p.Total)*1024*1024 - 3<<16

	// calculate partition sizes
	alpha := int(float64(total)*float64(p.Alpha)/100) >> 12 << 12
	beta := int(float64(total)*float64(p.Beta)/100) >> 12 << 12
	storage := int(float64(total) * float64(p.Storage) / 100)

	// replace template
	partitions = strings.ReplaceAll(partitions, "ALPHA_BYTES", strconv.Itoa(alpha))
	partitions = strings.ReplaceAll(partitions, "BETA_BYTES", strconv.Itoa(beta))
	partitions = strings.ReplaceAll(partitions, "STORAGE_BYTES", strconv.Itoa(storage))

	// print partitions
	println(partitions)

	return partitions, nil
}

// Build will build the project.
func Build(naosPath, target string, overrides map[string]string, files []string, partitions *Partitions, clean, reconfigure, appOnly bool, out io.Writer) error {
	// ensure target
	if target == "" {
		target = "esp32"
	}

	// prepare files contents
	var filesContent = "COMPONENT_EMBED_FILES :="
	var filesContent2 string
	for _, file := range files {
		filesContent += " data/" + file
		filesContent2 += "data/" + file
	}
	filesContent += "\n"

	// update files
	utils.Log(out, "Updating files...")
	err := os.WriteFile(filepath.Join(Directory(naosPath), "main", "files.list"), []byte(filesContent2), 0644)
	if err != nil {
		return err
	}

	// determine path
	configPath := filepath.Join(Directory(naosPath), "sdkconfig")
	overridesPath := filepath.Join(Directory(naosPath), "sdkconfig.overrides")

	// check if sdkconfig is layered
	isLayered, err := utils.Exists(filepath.Join(Directory(naosPath), "sdkconfig.defaults"))

	// apply overrides
	if len(overrides) > 0 {
		utils.Log(out, "Overriding sdkconfig...")

		// check if layered
		if isLayered {
			// update overrides file
			err = utils.Update(overridesPath, joinOverrides(overrides))
			if err != nil {
				return err
			}
		} else {
			// read config
			data, err := os.ReadFile(configPath)
			if err != nil {
				return err
			}

			// get config
			sdkconfig := string(data)

			// recreate if no match
			if !hasOverrides(sdkconfig, overrides) {
				// apply overrides
				sdkconfig, err = applyOverrides(naosPath, overrides)
				if err != nil {
					return err
				}

				// write config
				err = utils.Update(configPath, sdkconfig)
				if err != nil {
					return err
				}
			}
		}
	}

	// otherwise, ensure default sdkconfig
	if len(overrides) == 0 {
		utils.Log(out, "Ensure sdkconfig...")

		// check if layered
		if isLayered {
			// update overrides file
			err = utils.Update(overridesPath, "")
			if err != nil {
				return err
			}
		} else {
			// get original
			original, err := utils.Original(naosPath, filepath.Join("tree", "sdkconfig"))
			if err != nil {
				return err
			}

			// get existing
			existing, err := os.ReadFile(configPath)
			if err != nil {
				return err
			}

			// overwrite if changed
			if string(existing) != original {
				err = os.WriteFile(configPath, []byte(original), 0644)
				if err != nil {
					return err
				}
			}
		}
	}

	// check partitions
	if partitions != nil {
		// generate table
		table, err := partitions.generate()
		if err != nil {
			return err
		}

		// update partitions
		utils.Log(out, "Generating partitions...")
		err = utils.Update(filepath.Join(Directory(naosPath), "partitions.csv"), table)
		if err != nil {
			return err
		}
	}

	// sync partitions
	utils.Log(out, "Sync partitions...")
	partSrc := filepath.Join(Directory(naosPath), "main", "data", "partitions.csv")
	partDst := filepath.Join(Directory(naosPath), "partitions.csv")
	err = utils.Sync(partSrc, partDst)
	if err != nil {
		return err
	}

	// clean project if requested
	if clean {
		utils.Log(out, "Cleaning project...")
		err = Exec(naosPath, out, nil, false, false, "idf.py", "fullclean")
		if err != nil {
			return err
		}
	}

	// reconfigure if requested
	if reconfigure {
		utils.Log(out, "Reconfiguring project...")
		err = os.Remove(configPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		err = Exec(naosPath, out, nil, false, false, "idf.py", "-DIDF_TARGET="+target, "reconfigure")
		if err != nil {
			return err
		}
	}

	// build project (app only)
	if appOnly {
		utils.Log(out, "Building project (app only)...")
		err = Exec(naosPath, out, nil, false, false, "idf.py", "build", "app")
		if err != nil {
			return err
		}

		return nil
	}

	// build project
	utils.Log(out, "Building project...")
	err = Exec(naosPath, out, nil, false, false, "idf.py", "build")
	if err != nil {
		return err
	}

	return nil
}

// AppBinary will return the bytes of the built app binary.
func AppBinary(naosPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Directory(naosPath), "build", "naos-project.bin"))
}

func hasOverrides(sdkconfig string, overrides map[string]string) bool {
	// prepare scanner
	scanner := bufio.NewScanner(strings.NewReader(sdkconfig))

	// check lines
	for scanner.Scan() {
		// get line
		line := strings.TrimSpace(scanner.Text())

		// skip empty lines
		if line == "" {
			continue
		}

		// verify unset constants
		if strings.HasSuffix(line, "is not set") {
			name := line[2 : len(line)-11]
			if overrides[name] != "" {
				return false
			}
			continue
		}

		// ignore comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// verify set constants
		seg := strings.Split(line, "=")
		if _, ok := overrides[seg[0]]; !ok {
			continue
		}
		if seg[1] != overrides[seg[0]] {
			return false
		}
	}

	return true
}

func applyOverrides(naosPath string, overrides map[string]string) (string, error) {
	// read original config
	sdkconfig, err := utils.Original(naosPath, filepath.Join("tree", "sdkconfig"))
	if err != nil {
		return "", err
	}

	// check overrides
	if len(overrides) == 0 {
		return sdkconfig, nil
	}

	// append comments
	sdkconfig += "\n#\n# OVERRIDES\n#\n"

	// replace lines
	for key, value := range overrides {
		re := regexp.MustCompile("(?m)^(" + regexp.QuoteMeta(key) + ")=(.*)$")
		if re.MatchString(sdkconfig) {
			if value != "" {
				sdkconfig = re.ReplaceAllString(sdkconfig, key+"="+value)
			} else {
				sdkconfig = re.ReplaceAllString(sdkconfig, "# "+key+" is not set")
			}
		} else {
			if value != "" {
				sdkconfig += key + "=" + value + "\n"
			}
		}
	}

	return sdkconfig, nil
}

func joinOverrides(overrides map[string]string) string {
	sdkconfig := ""
	for key, value := range overrides {
		if value != "" {
			sdkconfig += key + "=" + value + "\n"
		}
	}
	return sdkconfig
}
