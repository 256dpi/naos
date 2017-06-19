package naos

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mholt/archiver"
)

// InventoryFileName specifies the inventory file name.
const InventoryFileName = "naos.json"

// HiddenDirectory specifies the hidden directory name.
const HiddenDirectory = ".naos"

// A Project is a project available on disk.
type Project struct {
	Location  string
	Inventory *Inventory
}

// CreateProject will initialize a project in the specified directory.
func CreateProject(path string) (*Project, error) {
	// ensure p directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	// create p
	p := &Project{
		Location:  path,
		Inventory: NewInventory(),
	}

	// save inventory
	err = p.SaveInventory()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// FindProject will look for project in the specified path.
func FindProject(path string) (*Project, error) {
	// attempt to read inventory
	inv, err := ReadInventory(filepath.Join(path, InventoryFileName))
	if err != nil {
		return nil, err
	}

	// TODO: Walk up the tree and try parent directories.

	// prepare project
	project := &Project{
		Location:  path,
		Inventory: inv,
	}

	return project, nil
}

// SaveInventory will save the associated inventory to disk.
func (p *Project) SaveInventory() error {
	// save inventory
	err := p.Inventory.Save(filepath.Join(p.Location, InventoryFileName))
	if err != nil {
		return err
	}

	return nil
}

// InstallToolchain will install the compilation toolchain.
func (p *Project) InstallToolchain(force bool) error {
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
	parentDir := filepath.Join(p.Location, HiddenDirectory)
	toolchainDir := filepath.Join(parentDir, "xtensa-esp32-elf")

	// check if already exists
	ok, err := exists(toolchainDir)
	if err != nil {
		return err
	}

	// return immediately if already exists and no force install is requested
	if ok && !force {
		return nil
	}

	// remove existing directory if existing
	if ok {
		err = os.RemoveAll(toolchainDir)
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
	err = download(tmp.Name(), url)
	if err != nil {
		return err
	}

	// unpack toolchain
	err = archiver.TarGz.Open(tmp.Name(), parentDir)
	if err != nil {
		return err
	}

	return nil
}

// ToolchainLocation returns the location of the toolchain if it exists or an
// empty string if it does not exist.
func (p *Project) ToolchainLocation() (string, error) {
	// calculate directory
	dir := filepath.Join(p.Location, HiddenDirectory, "xtensa-esp32-elf")

	// check if toolchain directory exists
	ok, err := exists(dir)
	if err != nil {
		return "", err
	}

	// return empty string if not existing
	if !ok {
		return "", nil
	}

	return dir, nil
}
