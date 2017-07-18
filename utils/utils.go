// Package utils provides some small utility functions.
package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

// Exists will check if the provided file or directory exists.
func Exists(path string) (bool, error) {
	// get file info
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	// check for known error
	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

// Download will download the specified source to the specified destination.
func Download(destination, source string) error {
	// create file
	file, err := os.Create(destination)
	if err != nil {
		return err
	}

	// make sure file gets closed
	defer file.Close()

	// read data
	resp, err := http.Get(source)
	if err != nil {
		return err
	}

	// make sure the body gets closed
	defer resp.Body.Close()

	// write body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	// properly close file
	err = file.Close()
	if err != nil {
		return err
	}

	return nil
}

// Clone will checkout the provided repository set it to the specified version
// and properly checkout all submodules.
func Clone(repo, path, commit string, out io.Writer) error {
	// construct clone command
	cmd := exec.Command("git", "clone", "--recursive", repo, path)
	cmd.Stdout = out
	cmd.Stderr = out

	// clone repo
	err := cmd.Run()
	if err != nil {
		return err
	}

	// construct reset command
	cmd = exec.Command("git", "reset", "--hard", commit)
	cmd.Stderr = out
	cmd.Stdout = out
	cmd.Dir = path

	// reset repo to specific version
	err = cmd.Run()
	if err != nil {
		return err
	}

	// construct update command
	cmd = exec.Command("git", "submodule", "update", "--recursive")
	cmd.Stderr = out
	cmd.Stdout = out
	cmd.Dir = path

	// set submodules to proper version
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Log will format and write the provided message to out if available.
func Log(out io.Writer, msg string) {
	if out != nil {
		fmt.Fprintf(out, "==> %s\n", msg)
	}
}
