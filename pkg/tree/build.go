package tree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

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
	if p.Beta == 0 {
		partitions = `# Name,   Type, SubType,  Offset,  Size
nvs,      data, nvs,      0x9000,  0x4000
otadata,  data, ota,      0xd000,  0x2000
phy_init, data, phy,      0xf000,  0x1000
alpha,    app,  factory,    0x10000, ALPHA_BYTES
storage,  data, fat,      ,        STORAGE_BYTES
coredump, data, coredump, ,        64K
`
	}

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

	return partitions, nil
}

// Build will build the project.
func Build(naosPath, appName, tagPrefix, target string, overrides map[string]string, files []string, partitions *Partitions, clean, reconfigure, appOnly bool, out io.Writer) error {
	// ensure target
	if target == "" {
		target = "esp32"
	}

	// update project name
	var err error
	if appName != "" {
		_, err = utils.Update(filepath.Join(Directory(naosPath), "project-name.txt"), appName)
	} else {
		err = utils.Remove(filepath.Join(Directory(naosPath), "project-name.txt"))
	}
	if err != nil {
		return fmt.Errorf("failed to update project name: %w", err)
	}

	// update project version
	appVersion, err := utils.Describe(filepath.Join(Directory(naosPath), "main", "src"), tagPrefix)
	if err != nil {
		return fmt.Errorf("failed to describe app version: %w", err)
	}
	if appVersion != "" {
		_, err = utils.Update(filepath.Join(Directory(naosPath), "version.txt"), appVersion)
	} else {
		err = utils.Remove(filepath.Join(Directory(naosPath), "version.txt"))
	}

	// prepare files content (one file per line, as the list is read back
	// with "file(STRINGS ...)" which splits on newlines)
	var filesContent string
	for _, file := range files {
		filesContent += "data/" + file + "\n"
	}

	// update files
	utils.Log(out, "Updating files...")
	changedFiles, err := utils.Update(filepath.Join(Directory(naosPath), "main", "files.list"), filesContent)
	if err != nil {
		return err
	} else if changedFiles {
		reconfigure = true
	}

	// determine path
	configPath := filepath.Join(Directory(naosPath), "sdkconfig")
	overridesPath := filepath.Join(Directory(naosPath), "sdkconfig.overrides")

	// sync overrides
	utils.Log(out, "Syncing overrides...")
	changedOverrides, err := utils.Update(overridesPath, joinOverrides(overrides))
	if err != nil {
		return err
	} else if changedOverrides {
		reconfigure = true
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
		changedPartitions, err := utils.Update(filepath.Join(Directory(naosPath), "partitions.csv"), table)
		if err != nil {
			return err
		} else if changedPartitions {
			reconfigure = true
		}
	} else {
		// sync partitions
		utils.Log(out, "Sync partitions...")
		partSrc := filepath.Join(naosPath, "..", "partitions.csv")
		partDst := filepath.Join(Directory(naosPath), "partitions.csv")
		err = utils.Sync(partSrc, partDst)
		if err != nil {
			return err
		}
	}

	// check configured target
	configuredTarget, err := readConfiguredTarget(naosPath)
	if err != nil {
		return err
	} else if configuredTarget != "" && configuredTarget != target {
		// a configured build directory cannot be re-targeted in place
		utils.Log(out, fmt.Sprintf("Target changed from %s to %s...", configuredTarget, target))
		clean = true
		reconfigure = true
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
		err = utils.Remove(configPath)
		if err != nil {
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

// AppBinary will return the path to the built app binary.
func AppBinary(naosPath, appName string) string {
	// ensure app name
	if appName == "" {
		appName = "naos-project"
	}
	return filepath.Join(Directory(naosPath), "build", appName+".bin")
}

// AppELF will return the path to the built app ELF file.
func AppELF(naosPath, appName string) string {
	// ensure app name
	if appName == "" {
		appName = "naos-project"
	}

	return filepath.Join(Directory(naosPath), "build", appName+".elf")
}

// readConfiguredTarget will return the target the build directory has been
// configured for, or an empty string if it has not been configured yet.
func readConfiguredTarget(naosPath string) (string, error) {
	// read cmake cache
	data, err := os.ReadFile(filepath.Join(Directory(naosPath), "build", "CMakeCache.txt"))
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	// find target
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "IDF_TARGET:STRING="); ok {
			return strings.TrimSpace(value), nil
		}
	}

	return "", nil
}

func joinOverrides(overrides map[string]string) string {
	// collect and sort keys to get a stable order (an unstable order would
	// change the file on every build and force a needless reconfiguration)
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// compile config
	config := ""
	for _, key := range keys {
		if overrides[key] != "" {
			config += key + "=" + overrides[key] + "\n"
		}
	}

	return config
}
