package toolchain

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mholt/archiver"
)

// InstallCompiler will install the xtensa compiler. An existing compiler will
// be removed if force is set to true. If out is not nil, it will be used to log
// information about the installation process.
func InstallCompiler(parent string, force bool, out io.Writer) error {
	// get toolchain url
	var url string
	switch runtime.GOOS {
	case "darwin":
		url = "https://dl.espressif.com/dl/xtensa-esp32-elf-osx-1.22.0-61-gab8375a-5.2.0.tar.gz"
	case "linux":
		url = "https://dl.espressif.com/dl/xtensa-esp32-elf-linux64-1.22.0-61-gab8375a-5.2.0.tar.gz"
	default:
		return errors.New("unsupported os")
	}

	// prepare toolchain directory
	dir := filepath.Join(parent, "xtensa-esp32-elf")

	// check if already exists
	ok, err := exists(dir)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		log(out, "Skipping toolchain as it already exists.")
		return nil
	}

	// remove existing directory if existing
	if ok {
		log(out, "Removing existing toolchain (forced).")
		err = os.RemoveAll(dir)
		if err != nil {
			return err
		}
	}

	// get a temporary file
	tmp, err := ioutil.TempFile("", "naos")
	if err != nil {
		return err
	}

	// make sure temporary file gets closed
	defer tmp.Close()

	// download toolchain
	log(out, "Downloading toolchain...")
	err = download(tmp.Name(), url)
	if err != nil {
		return err
	}

	// unpack toolchain
	log(out, "Unpacking toolchain...")
	err = archiver.TarGz.Open(tmp.Name(), parent)
	if err != nil {
		return err
	}

	// close temporary file
	tmp.Close()

	// remove temporary file
	err = os.Remove(tmp.Name())
	if err != nil {
		return err
	}

	return nil
}

// CompilerBinDirectory returns the assumed location of the compiler binary
// directory.
//
// Note: It will not check if the directory exists.
func CompilerBinDirectory(parent string) string {
	return filepath.Join(parent, "xtensa-esp32-elf", "bin")
}
