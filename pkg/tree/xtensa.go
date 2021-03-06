package tree

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mholt/archiver"

	"github.com/256dpi/naos/pkg/utils"
)

// InstallToolchain will install the xtensa toolchain. An existing toolchain will
// be removed if force is set to true. If out is not nil, it will be used to log
// information about the installation process.
func InstallToolchain(naosPath, version string, force bool, out io.Writer) error {
	// get toolchain url
	var url string
	switch runtime.GOOS {
	case "darwin":
		url = "https://dl.espressif.com/dl/xtensa-esp32-elf-osx-" + version + ".tar.gz"
	case "linux":
		url = "https://dl.espressif.com/dl/xtensa-esp32-elf-linux64-" + version + ".tar.gz"
	default:
		return errors.New("unsupported os")
	}

	// prepare toolchain directory
	dir := filepath.Join(Directory(naosPath), "toolchain", version)

	// check if already exists
	ok, err := utils.Exists(dir)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		utils.Log(out, "Skipping xtensa toolchain as it already exists.")
		return nil
	}

	// remove existing directory if existing
	if ok {
		utils.Log(out, "Removing existing xtensa toolchain (forced).")
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
	utils.Log(out, "Downloading xtensa toolchain...")
	err = utils.Download(tmp.Name(), url)
	if err != nil {
		return err
	}

	// unpack toolchain
	utils.Log(out, "Unpacking xtensa toolchain...")
	err = archiver.TarGz.Open(tmp.Name(), dir)
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

// BinDirectory returns the assumed location of the xtensa toolchain 'bin'
// directory.
//
// Note: It will not check if the directory exists.
func BinDirectory(naosPath string) (string, error) {
	// get required toolchain version
	version, err := RequiredToolchain(naosPath)
	if err != nil {
		return "", err
	}

	return filepath.Join(Directory(naosPath), "toolchain", version, "xtensa-esp32-elf", "bin"), nil
}
