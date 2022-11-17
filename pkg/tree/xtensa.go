package tree

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mholt/archiver/v3"

	"github.com/256dpi/naos/pkg/utils"
)

// InstallToolchain will install the toolchain for v4+ projects. An existing
// toolchain will be removed if force is set to true. If out is not nil, it will
// be used to log information about the installation process.
func InstallToolchain(naosPath string, force bool, out io.Writer) error {
	// prepare tools path
	toolsPath := filepath.Join(Directory(naosPath), "toolchain")

	// check if already exists
	ok, err := utils.Exists(toolsPath)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		utils.Log(out, "Skipping toolchain as it already exists.")
		return nil
	}

	// remove existing directory if existing
	if ok {
		utils.Log(out, "Removing existing toolchain (forced).")
		err = os.RemoveAll(toolsPath)
		if err != nil {
			return err
		}
	}

	// prepare command
	cmd := exec.Command(filepath.Join(IDFDirectory(naosPath), "install.sh"), "esp32")

	// set working directory
	cmd.Dir = Directory(naosPath)

	// connect output and inputs
	cmd.Stdout = out
	cmd.Stderr = out

	// inherit current environment
	cmd.Env = os.Environ()

	// go through all env variables
	for i, str := range cmd.Env {
		if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + Directory(naosPath)
		}
	}

	// set IDF tools path
	cmd.Env = append(cmd.Env, "IDF_TOOLS_PATH="+toolsPath)

	// run command
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// InstallToolchain3 will install the toolchain for v3 projects. An existing
// toolchain will be removed if force is set to true. If out is not nil, it will
// be used to log information about the installation process.
func InstallToolchain3(naosPath, version string, force bool, out io.Writer) error {
	// get toolchain url
	var url string
	switch runtime.GOOS {
	case "darwin":
		url = "https://dl.espressif.com/dl/xtensa-esp32-elf-macos-" + version + ".tar.gz"
	case "linux":
		url = "https://dl.espressif.com/dl/xtensa-esp32-elf-linux64-" + version + ".tar.gz"
	default:
		return errors.New("unsupported os")
	}

	// prepare toolchain directory
	dir := filepath.Join(Directory(naosPath), "toolchain", version)

	// check if already exists
	ok, err := utils.Exists(dir)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		utils.Log(out, "Skipping xtensa toolchain as it already exists.")
		return nil
	}

	// remove existing directory if existing
	if ok {
		utils.Log(out, "Removing existing xtensa toolchain (forced).")
		err = os.RemoveAll(dir)
		if err != nil {
			return err
		}
	}

	// get a temporary file
	tmp, err := ioutil.TempFile("", "naos")
	if err != nil {
		return err
	}

	// make sure temporary file gets closed
	defer tmp.Close()

	// download toolchain
	utils.Log(out, "Downloading xtensa toolchain...")
	err = utils.Download(tmp.Name(), url)
	if err != nil {
		return err
	}

	// unpack toolchain
	utils.Log(out, "Unpacking xtensa toolchain...")
	err = archiver.DefaultTarGz.Unarchive(tmp.Name(), dir)
	if err != nil {
		return err
	}

	// close temporary file
	tmp.Close()

	// remove temporary file
	err = os.Remove(tmp.Name())
	if err != nil {
		return err
	}

	return nil
}

// BinDirectory returns the assumed location of the xtensa toolchain 'bin'
// directory.
//
// Note: It will not check if the directory exists.
func BinDirectory(naosPath string) (string, error) {
	// get IDF version
	v, err := IDFMajorVersion(naosPath)
	if err != nil {
		return "", err
	}

	// handle 4.x projects
	if v >= 4 {
		// get env variables
		var buf bytes.Buffer
		err = Exec(naosPath, &buf, nil, false, "env")
		if err != nil {
			println(err.Error())
			return "", err
		}

		// get env vars
		env := strings.Split(buf.String(), "\n")

		// extract toolchain path from path variables
		for _, str := range env {
			if strings.HasPrefix(str, "PATH=") {
				paths := filepath.SplitList(strings.TrimPrefix(str, "PATH="))
				for _, path := range paths {
					if strings.HasPrefix(path, filepath.Join(Directory(naosPath), "toolchain")) && strings.Contains(path, "xtensa") {
						return path, nil
					}
				}
			}
		}

		return "", fmt.Errorf("toolchain not found")
	}

	// get required toolchain version
	version, err := RequiredToolchain(naosPath)
	if err != nil {
		return "", err
	}

	return filepath.Join(Directory(naosPath), "toolchain", version, "xtensa-esp32-elf", "bin"), nil
}
