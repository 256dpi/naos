package tree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

// envCache is the name of the file that caches the ESP-IDF environment.
const envCache = "env.cache"

// envStampPrefix marks the first line of the cache, which records the ESP-IDF
// the cache has been written for.
const envStampPrefix = "#idf="

// envVolatile lists variables that must not be cached, either because they are
// artifacts of the shell used to source the export script or because they refer
// to temporary files that are cleaned up eventually.
var envVolatile = map[string]bool{
	"SHLVL":                    true,
	"_":                        true,
	"IDF_DEACTIVATE_FILE_PATH": true,
}

// baseEnv returns the environment used to run commands in the build tree,
// without the ESP-IDF specific additions.
func baseEnv(naosPath string) ([]string, error) {
	// inherit current environment
	env := os.Environ()

	// go through all env variables
	for i, str := range env {
		if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			env[i] = "PWD=" + Directory(naosPath)
		}
	}

	// add IDF tools path
	env = append(env, "IDF_TOOLS_PATH="+filepath.Join(Directory(naosPath), "toolchain"))

	// add managed components tweak
	env = append(env, "IDF_COMPONENT_OVERWRITE_MANAGED_COMPONENTS=1")

	// add ADF path if existing
	ok, err := utils.Exists(ADFDirectory(naosPath))
	if err != nil {
		return nil, err
	}
	if ok {
		env = append(env, "ADF_PATH="+ADFDirectory(naosPath))
	}

	return env, nil
}

// WriteEnvCache will source the ESP-IDF export script and cache the resulting
// environment in the build tree. Sourcing the script takes a few seconds, so
// the cache is what keeps subsequent commands fast.
func WriteEnvCache(naosPath string) error {
	// prepare base environment
	base, err := baseEnv(naosPath)
	if err != nil {
		return err
	}

	// source export script and dump the resulting environment, aborting if
	// sourcing fails to not cache an incomplete environment, and diverting its
	// output to stderr to keep it out of the dump but available for reporting
	source := filepath.Join(IDFDirectory(naosPath), "export.sh")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("source %q 1>&2 || exit 1; env -0", source))
	cmd.Dir = Directory(naosPath)
	cmd.Env = base
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to export ESP-IDF environment: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	// index base environment
	baseMap := map[string]string{}
	for _, str := range base {
		key, value, _ := strings.Cut(str, "=")
		baseMap[key] = value
	}

	// collect variables that have been added or changed
	var lines []string
	var exported bool
	for _, str := range strings.Split(stdout.String(), "\x00") {
		// skip empty entries
		if str == "" {
			continue
		}

		// split variable
		key, value, ok := strings.Cut(str, "=")
		if !ok || value == baseMap[key] || envVolatile[key] {
			continue
		}

		// store the prepended prefix only, to not freeze the outer path
		if key == "PATH" {
			value = strings.TrimSuffix(value, ":"+baseMap["PATH"])
		}

		// note the marker variable
		if key == "IDF_PATH" {
			exported = true
		}

		// collect variable
		lines = append(lines, key+"="+value)
	}

	// verify the environment, as the export script may return successfully
	// without having set anything up
	if !exported {
		return fmt.Errorf("incomplete ESP-IDF environment: %s", strings.TrimSpace(stderr.String()))
	}

	// prepend the stamp, so that a cache written for another ESP-IDF is not
	// mistaken for a current one
	stamp, err := envStamp(naosPath)
	if err != nil {
		return err
	}
	lines = append([]string{envStampPrefix + stamp}, lines...)

	// write cache
	err = os.WriteFile(filepath.Join(Directory(naosPath), envCache), []byte(strings.Join(lines, "\n")+"\n"), 0644)
	if err != nil {
		return err
	}

	return nil
}

// envStamp returns a value identifying the ESP-IDF the cache belongs to. The
// linked directory carries the version, so it changes whenever the tree is
// pointed at another installation.
func envStamp(naosPath string) (string, error) {
	// resolve linked ESP-IDF
	dir, err := filepath.EvalSymlinks(IDFDirectory(naosPath))
	if err != nil {
		return "", err
	}

	return dir, nil
}

// envCacheFresh reports whether the cache exists and was written for the
// currently linked ESP-IDF.
func envCacheFresh(naosPath, path string) (bool, error) {
	// read cache
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	// get stamp
	stamp, err := envStamp(naosPath)
	if err != nil {
		return false, err
	}

	// compare against the stamp on the first line
	line, _, _ := strings.Cut(string(data), "\n")

	return line == envStampPrefix+stamp, nil
}

// commandEnv returns the environment for a command run in the build tree,
// writing the environment cache first if it is missing or stale.
func commandEnv(naosPath string) ([]string, error) {
	// prepare base environment
	env, err := baseEnv(naosPath)
	if err != nil {
		return nil, err
	}

	// write cache if missing or stale
	path := filepath.Join(Directory(naosPath), envCache)
	fresh, err := envCacheFresh(naosPath, path)
	if err != nil {
		return nil, err
	} else if !fresh {
		err = WriteEnvCache(naosPath)
		if err != nil {
			return nil, err
		}
	}

	// read cache
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// apply cached variables
	for _, line := range strings.Split(string(data), "\n") {
		// skip empty lines and the stamp
		if line == "" || strings.HasPrefix(line, envStampPrefix) {
			continue
		}

		// split variable
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		// prepend the cached prefix to the current path
		if key == "PATH" {
			value = value + ":" + os.Getenv("PATH")
		}

		// apply variable
		env = append(env, key+"="+value)
	}

	return env, nil
}

// lookPath will find the named program in the provided environment. This is
// needed as exec.Command resolves programs using the current process' path.
func lookPath(name string, env []string) (string, error) {
	// return names that are already paths
	if strings.ContainsRune(name, os.PathSeparator) {
		return name, nil
	}

	// get path from environment
	var path string
	for _, str := range env {
		if value, ok := strings.CutPrefix(str, "PATH="); ok {
			path = value
		}
	}

	// search directories
	for _, dir := range filepath.SplitList(path) {
		file := filepath.Join(dir, name)
		if info, err := os.Stat(file); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return file, nil
		}
	}

	return "", fmt.Errorf("failed to find %q in the ESP-IDF environment", name)
}
