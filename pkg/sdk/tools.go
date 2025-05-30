package sdk

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

// InstallToolchain will install the esp-idf toolchain.
func InstallToolchain(version string, out io.Writer) (string, error) {
	// ensure base
	base, err := ensureBase()
	if err != nil {
		return "", err
	}

	// prepare IDF key
	idfKey := "esp-idf+" + version
	toolsKey := "toolchain+" + version

	// prepare tools path
	idfDir := filepath.Join(base, idfKey)
	toolsDir := filepath.Join(base, toolsKey)
	flagFile := filepath.Join(toolsDir, ".installed")

	// check flag
	ok, err := utils.Exists(flagFile)
	if err != nil {
		return "", err
	} else if ok {
		return toolsDir, nil
	}

	// prepare command
	cmd := exec.Command(filepath.Join(idfDir, "install.sh"), "all")

	// connect output and inputs
	cmd.Stdout = out
	cmd.Stderr = out

	// inherit current environment
	cmd.Env = os.Environ()

	// go through all env variables
	for i, str := range cmd.Env {
		if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + toolsDir
		}
	}

	// set IDF tools path
	cmd.Env = append(cmd.Env, "IDF_TOOLS_PATH="+toolsDir)

	// run command
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	// write flag
	err = os.WriteFile(flagFile, []byte{}, 0644)
	if err != nil {
		return "", err
	}

	return toolsDir, nil
}
