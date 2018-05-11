package utils

import (
	"io"
	"net/http"
	"os"
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
