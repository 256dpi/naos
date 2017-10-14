package tree

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

// ParseCoredump will parse the provided raw coredump data and return a human
// readable representation.
func ParseCoredump(treePath string, coredump []byte) (string, error) {
	// get paths
	espCoredump := filepath.Join(IDFDirectory(treePath), "components", "espcoredump", "espcoredump.py")
	projectELF := filepath.Join(treePath, "build", "naos-project.elf")

	// get a temporary file
	file, err := ioutil.TempFile("", "coredump")
	if err != nil {
		return "", err
	}

	// ensure file gets closed
	defer file.Close()

	// write core dump to file
	_, err = file.Write(coredump)
	if err != nil {
		return "", err
	}

	// close file
	err = file.Close()
	if err != nil {
		return "", err
	}

	// create buffer
	buf := new(bytes.Buffer)

	// parse coredump
	err = Exec(treePath, buf, nil, espCoredump, "info_corefile", "-t", "raw", "-c", file.Name(), projectELF)
	if err != nil {
		return "", err
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
