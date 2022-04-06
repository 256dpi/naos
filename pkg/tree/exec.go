package tree

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/256dpi/naos/pkg/utils"
)

// IDFDirectory returns the assumed location of the esp-idf directory.
//
// Note: It will not check if the directory exists.
func IDFDirectory(naosPath string) string {
	return filepath.Join(Directory(naosPath), "esp-idf")
}

// Exec runs a named command in the build tree. All xtensa toolchain binaries are
// made available in the path transparently.
func Exec(naosPath string, out io.Writer, in io.Reader, name string, arg ...string) error {
	// print command
	utils.Log(out, fmt.Sprintf("%s %s", name, strings.Join(arg, " ")))

	// get major IDF version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// prepare command
	var cmd *exec.Cmd

	// construct command for v3 projects
	if idfMajorVersion == 3 {
		cmd = exec.Command(name, arg...)
	}

	// construct command for v4 projects
	if idfMajorVersion == 4 {
		source := filepath.Join(IDFDirectory(naosPath), "export.sh")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("source %s; %s %s", source, name, strings.Join(arg, " ")))
		println(fmt.Sprintf("source %s; %s %s", source, name, strings.Join(arg, " ")))
	}

	// set working directory
	cmd.Dir = Directory(naosPath)

	// connect output and inputs
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = in

	// inherit current environment
	cmd.Env = os.Environ()

	// go through all env variables
	for i, str := range cmd.Env {
		if idfMajorVersion == 3 && strings.HasPrefix(str, "PATH=") {
			// get bin directory
			bin, err := BinDirectory(naosPath)
			if err != nil {
				return err
			}

			// prepend toolchain bin directory for v3 projects
			cmd.Env[i] = "PATH=" + bin + ":" + os.Getenv("PATH")
		} else if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + Directory(naosPath)
		}
	}

	// add IDF path for v3 projects
	if idfMajorVersion == 3 {
		cmd.Env = append(cmd.Env, "IDF_PATH="+IDFDirectory(naosPath))
	}

	// add IDF tools path for v4 projects
	if idfMajorVersion == 4 {
		cmd.Env = append(cmd.Env, "IDF_TOOLS_PATH="+filepath.Join(Directory(naosPath), "toolchain"))
	}

	// run command
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
