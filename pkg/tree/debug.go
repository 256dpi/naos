package tree

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

// ParseCoredump will parse the provided raw coredump data and return a
// human-readable representation.
func ParseCoredump(naosPath string, coredump []byte) ([]byte, error) {
	// get paths
	espCoredump := filepath.Join(IDFDirectory(naosPath), "components", "espcoredump", "espcoredump.py")
	projectELF := filepath.Join(Directory(naosPath), "build", "naos-project.elf")

	// get a temporary file
	file, err := ioutil.TempFile("", "coredump")
	if err != nil {
		return nil, err
	}

	// ensure file gets closed
	defer file.Close()

	// write core dump to file
	_, err = file.Write(coredump)
	if err != nil {
		return nil, err
	}

	// close file
	err = file.Close()
	if err != nil {
		return nil, err
	}

	// create buffer
	buf := new(bytes.Buffer)

	// parse coredump
	err = Exec(naosPath, buf, nil, espCoredump, "info_corefile", "-t", "raw", "-c", file.Name(), projectELF)
	if err != nil {
		return nil, err
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
