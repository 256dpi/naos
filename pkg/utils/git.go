package utils

import (
	"io"
	"os/exec"
	"strings"
)

// Ref will return the current branch or tag name of the repository.
func Ref(path string) (string, error) {
	// get current branch or tag name
	cmd := exec.Command("git", "symbolic-ref", "-q", "--short", "HEAD")
	cmd.Dir = path
	buf, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "describe", "--tags", "--exact-match")
		cmd.Dir = path
		buf, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}

	return string(buf), nil
}

// Clone will check out the provided repository set it to the specified version
// and properly checkout all submodules.
func Clone(repo, path, commit string, ignoredSubmodules []string, out io.Writer) error {
	// clone repo
	cmd := exec.Command("git", "clone", repo, path)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		return err
	}

	// run fetch
	err = Fetch(path, commit, ignoredSubmodules, out)
	if err != nil {
		return err
	}

	return nil
}

// Fetch will update the remote repository and update all submodules
// accordingly.
func Fetch(path, commit string, ignoredSubmodules []string, out io.Writer) error {
	// unprefix commit
	_commit := commit
	if strings.HasPrefix(commit, "origin/") {
		_commit = commit[7:]
	}

	// fetch repo
	cmd := exec.Command("git", "fetch", "origin", _commit)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Dir = path
	err := cmd.Run()
	if err != nil {
		return err
	}

	// reset repo to specific version
	cmd = exec.Command("git", "reset", "--hard", commit)
	cmd.Stderr = out
	cmd.Stdout = out
	cmd.Dir = path
	err = cmd.Run()
	if err != nil {
		return err
	}

	// remove not needed submodules
	for _, ism := range ignoredSubmodules {
		cmd = exec.Command("git", "rm", "-rfq", ism)
		cmd.Stderr = out
		cmd.Stdout = out
		cmd.Dir = path
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	// set submodules to proper version
	cmd = exec.Command("git", "submodule", "update", "--recursive", "--init")
	cmd.Stderr = out
	cmd.Stdout = out
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

// Apply will apply the specified patch to the provided repository.
func Apply(repo, patch string, out io.Writer) error {
	// check if patch has been already applied
	cmd := exec.Command("git", "apply", patch, "--check", "--reverse")
	cmd.Dir = repo
	err := cmd.Run()
	if err == nil {
		return nil
	}

	// apply patch
	cmd = exec.Command("git", "apply", patch)
	cmd.Dir = repo
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
