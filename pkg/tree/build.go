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
	if err != nil {
		return fmt.Errorf("failed to update project version: %w", err)
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

	// read cmake cache
	cache, err := readCMakeCache(naosPath)
	if err != nil {
		return err
	}

	// check configured target
	if configured := cache["IDF_TARGET"]; configured != "" && configured != target {
		// a configured build directory cannot be re-targeted in place
		utils.Log(out, fmt.Sprintf("Target changed from %s to %s...", configured, target))
		clean = true
		reconfigure = true
	}

	// check configured toolchain, as a build directory configured for another
	// ESP-IDF version or toolchain refers to files that have been replaced
	stale, err := staleToolchain(naosPath, cache)
	if err != nil {
		return err
	} else if stale {
		utils.Log(out, "Toolchain changed...")
		clean = true
		reconfigure = true
	}

	// clean project if requested
	if clean {
		utils.Log(out, "Cleaning project...")
		err = cleanProject(naosPath, out)
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

// readCMakeCache will return the entries of the build directories cmake cache,
// or an empty map if it has not been configured yet.
func readCMakeCache(naosPath string) (map[string]string, error) {
	// prepare cache
	cache := map[string]string{}

	// read cmake cache
	data, err := os.ReadFile(filepath.Join(Directory(naosPath), "build", "CMakeCache.txt"))
	if os.IsNotExist(err) {
		return cache, nil
	} else if err != nil {
		return nil, err
	}

	// collect entries ("KEY:TYPE=VALUE")
	for _, line := range strings.Split(string(data), "\n") {
		// split entry
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		// strip type
		key, _, ok := strings.Cut(name, ":")
		if !ok {
			continue
		}

		// store entry
		cache[key] = strings.TrimSpace(value)
	}

	return cache, nil
}

// staleToolchain will check whether the build directory has been configured
// with an ESP-IDF version or toolchain that is not the current one anymore.
func staleToolchain(naosPath string, cache map[string]string) (bool, error) {
	// check the linked ESP-IDF directory against the configured toolchain
	// file, as both versions may still be installed side by side
	if file := cache["CMAKE_TOOLCHAIN_FILE"]; file != "" {
		idfDir, err := filepath.EvalSymlinks(IDFDirectory(naosPath))
		if err == nil && !strings.HasPrefix(file, idfDir+string(filepath.Separator)) {
			return true, nil
		}
	}

	// check that the configured compiler still exists, as the toolchain may
	// have been updated in place
	if compiler := cache["CMAKE_C_COMPILER_AR"]; compiler != "" {
		ok, err := utils.Exists(compiler)
		if err != nil {
			return false, err
		} else if !ok {
			return true, nil
		}
	}

	return false, nil
}

// cleanProject will clean the build directory.
func cleanProject(naosPath string, out io.Writer) error {
	// let idf.py clean the build directory
	err := Exec(naosPath, out, nil, false, false, "idf.py", "fullclean")
	if err == nil {
		return nil
	}

	// otherwise remove the build directory, as idf.py refuses to clean
	// directories it does not recognize as cmake build directories
	utils.Log(out, "Removing build directory...")

	return os.RemoveAll(filepath.Join(Directory(naosPath), "build"))
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
