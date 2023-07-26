package tree

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

// TODO: Changing embeds in a v4 projects requires a clean.
// TODO: Updating sdkconfig.overrides requires a reconfigure.

// Build will build the project.
func Build(naosPath string, overrides map[string]string, files []string, clean, reconfigure, appOnly bool, out io.Writer) error {
	// get idf major version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
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
	if idfMajorVersion == 3 {
		err = os.WriteFile(filepath.Join(Directory(naosPath), "main", "files.mk"), []byte(filesContent), 0644)
		if err != nil {
			return err
		}
	} else {
		err = os.WriteFile(filepath.Join(Directory(naosPath), "main", "files.list"), []byte(filesContent2), 0644)
		if err != nil {
			return err
		}
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
		if idfMajorVersion >= 4 {
			err = Exec(naosPath, out, nil, false, "idf.py", "clean")
		} else {
			err = Exec(naosPath, out, nil, false, "make", "clean")
		}
		if err != nil {
			return err
		}
	}

	// reconfigure if requested
	if reconfigure {
		utils.Log(out, "Reconfiguring project...")
		err = os.Remove(configPath)
		if err != nil {
			return err
		}
		err = Exec(naosPath, out, nil, false, "idf.py", "reconfigure")
		if err != nil {
			return err
		}
	}

	// build project (app only)
	if appOnly {
		utils.Log(out, "Building project (app only)...")
		if idfMajorVersion >= 4 {
			err = Exec(naosPath, out, nil, false, "idf.py", "build", "app")
		} else {
			err = Exec(naosPath, out, nil, false, "make", "app")
		}
		if err != nil {
			return err
		}

		return nil
	}

	// build project
	utils.Log(out, "Building project...")
	if idfMajorVersion >= 4 {
		err = Exec(naosPath, out, nil, false, "idf.py", "build")
	} else {
		err = Exec(naosPath, out, nil, false, "make", "all")
	}
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
