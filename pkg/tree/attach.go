package tree

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kr/pty"

	"github.com/256dpi/naos/pkg/utils"
)

// Attach will attach to the specified serial port using either miniterm in simple
// mode or idf_monitor.
func Attach(naosPath, port string, simple bool, out io.Writer, in io.Reader) error {
	// prepare command
	var cmd *exec.Cmd

	// set simple or advanced command
	if simple {
		// construct command
		cmd = exec.Command("miniterm.py", "--rts", "0", "--dtr", "0", "--raw", port, "115200")
	} else {
		// get path of monitor tool
		tool := filepath.Join(IDFDirectory(naosPath), "tools", "idf_monitor.py")

		// get elf path
		elf := filepath.Join(Directory(naosPath), "build", "naos-project.elf")

		// construct command
		cmd = exec.Command("python", tool, "--baud", "115200", "--port", port, elf)
	}

	// set working directory
	cmd.Dir = Directory(naosPath)

	// inherit current environment
	cmd.Env = os.Environ()

	// get bin directory
	bin, err := BinDirectory(naosPath)
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
			cmd.Env[i] = "PWD=" + Directory(naosPath)
		}
	}

	// start process and get tty
	utils.Log(out, "Attaching to serial port (press Ctrl+C to exit)...")
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	// make sure tty gets closed
	defer tty.Close()

	// prepare channel
	quit := make(chan struct{})

	// read data until EOF
	go func() {
		io.Copy(out, tty)
		close(quit)
	}()

	// write data until EOF
	go func() {
		io.Copy(tty, in)
		close(quit)
	}()

	// wait for quit
	<-quit

	return nil
}
