package utils

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Exists will check if the provided file or directory exists.
func Exists(path string) (bool, error) {
	// get file info
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}

	// check for known error
	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

// Resolve will resolve a path and us the provided base for relative paths.
func Resolve(path, base string) (string, error) {
	// only clean already absolute paths
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// ensure base
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// return path joined with base
	return filepath.Join(base, path), nil
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

// Sync will synchronize the two files. It will only copy if the destination is
// absent or differs.
func Sync(src, dst string) error {
	// check existence
	srcExists, err := Exists(src)
	if err != nil {
		return err
	}
	dstExists, err := Exists(dst)
	if err != nil {
		return err
	}

	// skip if source is missing
	if !srcExists {
		return nil
	}

	// read source
	srcData, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// check similarity if destination exists
	if dstExists {
		// read destination
		dstData, err := os.ReadFile(dst)
		if err != nil {
			return err
		}

		// check similarity
		if bytes.Compare(srcData, dstData) == 0 {
			return nil
		}
	}

	// write file
	err = os.WriteFile(dst, srcData, 0644)
	if err != nil {
		return err
	}

	return nil
}

// Update will write a file if the contents have changed.
func Update(path, content string) error {
	// read file
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// check if content has changed
	if data != nil && strings.TrimSpace(string(data)) == strings.TrimSpace(content) {
		return nil
	}

	// write file
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return err
	}

	return nil
}

// Link will ensure a link with the specified target at the provided path.
func Link(path, target string) error {
	// check path
	tgt, err := os.Readlink(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// check target
	if tgt == target {
		return nil
	}

	// remove link
	if tgt != "" {
		err = os.Remove(path)
		if err != nil {
			return err
		}
	}

	// make link
	err = os.Symlink(target, path)
	if err != nil {
		return err
	}

	return nil
}
