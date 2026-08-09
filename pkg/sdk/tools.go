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

	// prepare commands
	var commands [][]string

	// install toolchain if not yet installed
	installed, err := utils.Exists(flagFile)
	if err != nil {
		return "", err
	} else if !installed {
		commands = append(commands, []string{filepath.Join(idfDir, "install.sh"), "all"})
	}

	// install ninja if missing ("ninja" is an on request tool and thus not
	// covered by the install script, but yields notably faster builds than
	// the "Unix Makefiles" fallback)
	ninja, err := utils.Exists(filepath.Join(toolsDir, "tools", "ninja"))
	if err != nil {
		return "", err
	} else if !ninja {
		commands = append(commands, []string{filepath.Join(idfDir, "tools", "idf_tools.py"), "install", "ninja"})
	}

	// return early if there is nothing to do
	if len(commands) == 0 {
		return toolsDir, nil
	}

	// run commands
	for _, command := range commands {
		// prepare command
		cmd := exec.Command(command[0], command[1:]...)

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
	}

	// write flag
	err = os.WriteFile(flagFile, []byte{}, 0644)
	if err != nil {
		return "", err
	}

	return toolsDir, nil
}
