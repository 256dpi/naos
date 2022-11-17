package utils

import (
	"io"
	"os/exec"
	"strings"
)

// Clone will check out the provided repository set it to the specified version
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
	cmd = exec.Command("git", "submodule", "update", "--recursive", "--init", "-f")
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

// Fetch will update the remote repository and update all submodules
// accordingly.
func Fetch(path, commit string, out io.Writer) error {
	// construct fetch command
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = path

	// clone repo
	err := cmd.Run()
	if err != nil {
		return err
	}

	// prepend origin if not tag
	if !strings.HasPrefix(commit, "v") {
		commit = "origin/" + commit
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
	cmd = exec.Command("git", "submodule", "update", "--recursive", "--init")
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

// Replace will replace the specified git repository with the specified branch
// of the provided remote repository.
func Replace(path, remote, name, branch string, out io.Writer) error {
	// list remotes
	cmd := exec.Command("git", "remote")
	cmd.Dir = path
	buf, err := cmd.Output()
	if err != nil {
		return err
	}

	// add remote
	if !strings.Contains(string(buf), "fork") {
		cmd = exec.Command("git", "remote", "add", name, remote)
		cmd.Stdout = out
		cmd.Stderr = out
		cmd.Dir = path
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	// fetch remote
	cmd = exec.Command("git", "fetch", name)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = path
	err = cmd.Run()
	if err != nil {
		return err
	}

	// checkout branch
	cmd = exec.Command("git", "checkout", name+"/"+branch)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = path
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Original will return the original, unmodified contents of the specified file.
func Original(path, name string) (string, error) {
	// get file
	cmd := exec.Command("git", "show", "HEAD^^^:"+name)
	cmd.Dir = path
	buf, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(buf), nil
}
