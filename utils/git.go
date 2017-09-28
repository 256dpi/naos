package utils

import (
	"io"
	"os/exec"
)

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
	cmd = exec.Command("git", "reset", "--hard", "origin/"+commit)
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

// Fetch will updates to the remote repository and update all submodules
// accordingly.
func Fetch(path, commit string, out io.Writer) error {
	// construct fetch command
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Stdout = out
	cmd.Stderr = out

	// clone repo
	err := cmd.Run()
	if err != nil {
		return err
	}

	// construct reset command
	cmd = exec.Command("git", "reset", "--hard", "origin/"+commit)
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
