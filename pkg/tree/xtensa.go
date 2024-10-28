package tree

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	cmd := exec.Command(filepath.Join(IDFDirectory(naosPath), "install.sh"), "all")

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
