package tree

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"

	"github.com/256dpi/naos/pkg/utils"
)

// IDFDirectory returns the assumed location of the esp-idf directory.
//
// Note: It will not check if the directory exists.
func IDFDirectory(naosPath string) string {
	return filepath.Join(Directory(naosPath), "esp-idf")
}

// ADFDirectory returns the assumed location of the esp-adf directory.
//
// Note: It will not check if the directory exists.
func ADFDirectory(naosPath string) string {
	return filepath.Join(Directory(naosPath), "esp-adf")
}

// Exec runs a named command in the build tree. All xtensa toolchain binaries are
// made available in the path transparently.
func Exec(naosPath string, out io.Writer, in io.Reader, noEnv, usePty bool, name string, arg ...string) error {
	// print command
	utils.Log(out, fmt.Sprintf("%s %s", name, strings.Join(arg, " ")))

	// get major IDF version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// prepare command
	var cmd = exec.Command(name, arg...)

	// construct command for new projects
	if idfMajorVersion >= 4 && !noEnv {
		source := filepath.Join(IDFDirectory(naosPath), "export.sh")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("source %s; %s %s", source, name, strings.Join(arg, " ")))
	}

	// set working directory
	cmd.Dir = Directory(naosPath)

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

	// add IDF tools path for new projects
	if idfMajorVersion >= 4 {
		cmd.Env = append(cmd.Env, "IDF_TOOLS_PATH="+filepath.Join(Directory(naosPath), "toolchain"))
	}

	// add ADF path if existing
	ok, err := utils.Exists(ADFDirectory(naosPath))
	if err != nil {
		return err
	}
	if ok {
		cmd.Env = append(cmd.Env, "ADF_PATH="+ADFDirectory(naosPath))
	}

	// run command without PTY
	if !usePty {
		// connect input and outputs
		cmd.Stdin = in
		cmd.Stdout = out
		cmd.Stderr = out

		// run command
		err = cmd.Run()
		if err != nil {
			return err
		}

		return nil
	}

	/* run command with PTY */

	// run command
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	// make sure tty gets closed
	defer tty.Close()

	// prepare channel
	quit := make(chan os.Signal, 1)

	// read data until EOF
	go func() {
		_, _ = io.Copy(out, tty)
		quit <- os.Interrupt
	}()

	// write data until EOF
	go func() {
		_, _ = io.Copy(tty, in)
		quit <- os.Interrupt
	}()

	// handle interrupts
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// wait for interrupt
	<-quit

	return nil
}
