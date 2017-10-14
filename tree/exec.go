package tree

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IDFDirectory returns the assumed location of the esp-idf directory.
//
// Note: It will not check if the directory exists.
func IDFDirectory(treePath string) string {
	return filepath.Join(treePath, "esp-idf")
}

// Exec runs a named command in the tree. All xtensa toolchain binaries are
// made available in the path transparently.
func Exec(treePath string, out io.Writer, in io.Reader, name string, arg ...string) error {
	// construct command
	cmd := exec.Command(name, arg...)

	// set working directory
	cmd.Dir = treePath

	// connect output and inputs
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = in

	// inherit current environment
	cmd.Env = os.Environ()

	// get bin directory
	bin, err := BinDirectory(treePath)
	if err != nil {
		return err
	}

	// go through all env variables
	for i, str := range cmd.Env {
		if strings.HasPrefix(str, "PATH=") {
			// prepend toolchain bin directory
			cmd.Env[i] = "PATH=" + bin + ":" + os.Getenv("PATH")
		} else if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + treePath
		}
	}

	// add idf path
	cmd.Env = append(cmd.Env, "IDF_PATH=" + IDFDirectory(treePath))

	// run command
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
