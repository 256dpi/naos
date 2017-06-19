package naos

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

func exists(path string) (bool, error) {
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

func download(path, url string) error {
	// create file
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	// make sure file gets closed
	defer file.Close()

	// read data
	resp, err := http.Get(url)
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

func clone(repo, path, commit string, out io.Writer) error {
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

func log(out io.Writer, msg string) {
	if out != nil {
		fmt.Fprint(out, msg)
	}
}
