package utils

import (
	"bytes"
	"io"
	"net/http"
	"os"
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
