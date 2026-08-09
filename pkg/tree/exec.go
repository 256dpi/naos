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
	"golang.org/x/term"

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

	// prepare environment and program
	var err error
	var env []string
	program := name
	if noEnv {
		env, err = baseEnv(naosPath)
	} else {
		env, err = commandEnv(naosPath)
		if err == nil {
			program, err = lookPath(name, env)
		}
	}
	if err != nil {
		return err
	}

	// enable ccache if available
	if _, e := exec.LookPath("ccache"); e == nil {
		env = append(env, "IDF_CCACHE_ENABLE=1")
	}

	// prepare command
	cmd := exec.Command(program, arg...)

	// set working directory
	cmd.Dir = Directory(naosPath)

	// set environment
	cmd.Env = env

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

	// put the terminal in raw mode and forward its size, so that programs like
	// the ESP-IDF monitor receive keys like Ctrl+] and Ctrl+T unaltered and can
	// lay out their output correctly
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		// enable raw mode
		state, err := term.MakeRaw(int(file.Fd()))
		if err != nil {
			return err
		}
		defer term.Restore(int(file.Fd()), state)

		// inherit size and keep it up to date
		resize := make(chan os.Signal, 1)
		signal.Notify(resize, syscall.SIGWINCH)
		defer signal.Stop(resize)
		go func() {
			for range resize {
				_ = pty.InheritSize(file, tty)
			}
		}()
		resize <- syscall.SIGWINCH
	}

	// prepare channels
	quit := make(chan os.Signal, 1)
	drained := make(chan struct{})

	// handle interrupts
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	// read data until EOF
	go func() {
		_, _ = io.Copy(out, tty)
		close(drained)
	}()

	// write data until EOF
	go func() {
		_, _ = io.Copy(tty, in)
	}()

	// wait for the output to drain or an interrupt
	var interrupted bool
	select {
	case <-drained:
	case <-quit:
		// forward the interrupt and let the command wind down
		interrupted = true
		_ = cmd.Process.Signal(syscall.SIGINT)
		<-drained
	}

	// reap command and report its result unless it was interrupted
	err = cmd.Wait()
	if interrupted {
		return nil
	}

	return err
}
