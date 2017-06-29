package naos

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kr/pty"
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
	// ensure project directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	// create project
	p := &Project{
		Location:  path,
		Inventory: NewInventory(),
	}

	// save inventory
	err = p.SaveInventory()
	if err != nil {
		return nil, err
	}

	// ensure source directory
	err = os.MkdirAll(filepath.Join(path, "src"), 0755)
	if err != nil {
		return nil, err
	}

	// prepare main source path and check if it already exists
	mainSourcePath := filepath.Join(path, "src", "main.c")
	ok, err := exists(mainSourcePath)
	if err != nil {
		return nil, err
	}

	// create main source file if it not already exists
	if !ok {
		err = ioutil.WriteFile(mainSourcePath, []byte(mainSourceFile), 0644)
		if err != nil {
			return nil, err
		}
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

// HiddenDirectory returns the hidden used to store the toolchain, development
// framework and other necessary files.
func (p *Project) HiddenDirectory() string {
	return filepath.Join(p.Location, HiddenDirectory)
}

// SetupToolchain will setup the compilation toolchain. An existing toolchain
// will be removed if force is set to true. If out is not nil, it will be used
// to log information about the process.
func (p *Project) SetupToolchain(force bool, out io.Writer) error {
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
	toolchainDir := filepath.Join(p.HiddenDirectory(), "xtensa-esp32-elf")

	// check if already exists
	ok, err := exists(toolchainDir)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		log(out, fmt.Sprintln("Skipping toolchain as it already exists"))
		return nil
	}

	// remove existing directory if existing
	if ok {
		log(out, fmt.Sprintln("Removing existing toolchain (force=true)"))
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
	log(out, fmt.Sprintf("Downloading toolchain from '%s'...\n", url))
	err = download(tmp.Name(), url)
	if err != nil {
		return err
	}

	// unpack toolchain
	log(out, fmt.Sprintf("Unpacking toolchain to '%s'...\n", p.HiddenDirectory()))
	err = archiver.TarGz.Open(tmp.Name(), p.HiddenDirectory())
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

// ToolchainLocation returns the location of the toolchain if it exists or an
// error if it does not exist.
func (p *Project) ToolchainLocation() (string, error) {
	// calculate directory
	dir := filepath.Join(p.HiddenDirectory(), "xtensa-esp32-elf")

	// check if toolchain directory exists
	ok, err := exists(dir)
	if err != nil {
		return "", err
	} else if !ok {
		return "", errors.New("toolchain not found")
	}

	return dir, nil
}

// SetupDevelopmentFramework will setup the development framework. An existing
// development framework will be removed if force is set to true. If out is not
// nil, it will be used to log information about the process.
func (p *Project) SetupDevelopmentFramework(force bool, out io.Writer) error {
	// prepare toolchain directory
	frameworkDir := filepath.Join(p.HiddenDirectory(), "esp-idf")

	// check if already exists
	ok, err := exists(frameworkDir)
	if err != nil {
		return err
	}

	// return immediately if already exists and not forced
	if ok && !force {
		log(out, fmt.Sprintln("Skipping development framework as it already exists"))
		return nil
	}

	// remove existing directory if existing
	if ok {
		log(out, fmt.Sprintln("Removing existing development framework (force=true)"))
		err = os.RemoveAll(frameworkDir)
		if err != nil {
			return err
		}
	}

	// clone development framework
	err = clone("https://github.com/espressif/esp-idf.git", frameworkDir, ESPIDFVersion, out)
	if err != nil {
		return err
	}

	return nil
}

// DevelopmentFrameworkLocation returns the location of the development
// framework if it exists or an error if it does not exist.
func (p *Project) DevelopmentFrameworkLocation() (string, error) {
	// calculate directory
	dir := filepath.Join(p.HiddenDirectory(), "esp-idf")

	// check if toolchain directory exists
	ok, err := exists(dir)
	if err != nil {
		return "", err
	} else if !ok {
		return "", errors.New("development framework not found")
	}

	return dir, nil
}

// SetupBuildTree will setup the build tree. An existing build tree will be
// removed if force is set to true. If out is not nil, it will be used to log
// information about the process.
func (p *Project) SetupBuildTree(force bool, out io.Writer) error {
	// prepare build tree directory
	buildTreeDir := filepath.Join(p.HiddenDirectory(), "tree")

	// check if already exists
	ok, err := exists(buildTreeDir)
	if err != nil {
		return err
	}

	// return immediately if already exists and no forced
	if ok && !force {
		log(out, fmt.Sprintln("Skipping build tree as it already exists"))
		return nil
	}

	// remove existing directory if existing
	if ok {
		log(out, fmt.Sprintln("Removing existing build tree (force=true)"))
		err = os.RemoveAll(buildTreeDir)
		if err != nil {
			return err
		}
	}

	// adding directory
	log(out, fmt.Sprintf("Adding build tree directory to '%s'\n", buildTreeDir))
	err = os.MkdirAll(buildTreeDir, 0777)
	if err != nil {
		return err
	}

	// adding sdk config file
	log(out, fmt.Sprintf("Adding 'sdkconfig' to '%s'\n", buildTreeDir))
	err = ioutil.WriteFile(filepath.Join(buildTreeDir, "sdkconfig"), []byte(sdkconfigFile), 0644)
	if err != nil {
		return err
	}

	// adding makefile
	log(out, fmt.Sprintf("Adding 'Makefile' to '%s'\n", buildTreeDir))
	err = ioutil.WriteFile(filepath.Join(buildTreeDir, "Makefile"), []byte(makeFile), 0644)
	if err != nil {
		return err
	}

	// adding main directory
	log(out, fmt.Sprintf("Adding 'main' directory to '%s'\n", buildTreeDir))
	err = os.MkdirAll(filepath.Join(buildTreeDir, "main"), 0777)
	if err != nil {
		return err
	}

	// adding component.mk
	log(out, fmt.Sprintf("Adding 'main/component.mk' to '%s'\n", buildTreeDir))
	err = ioutil.WriteFile(filepath.Join(buildTreeDir, "main", "component.mk"), []byte(componentFile), 0644)
	if err != nil {
		return err
	}

	// linking src
	log(out, fmt.Sprintln("Linking 'src' directory"))
	err = os.Symlink(filepath.Join(p.Location, "src"), filepath.Join(buildTreeDir, "main", "src"))
	if err != nil {
		return err
	}

	// adding components directory
	log(out, fmt.Sprintf("Adding 'components' directory to '%s'\n", buildTreeDir))
	err = os.MkdirAll(filepath.Join(buildTreeDir, "components"), 0777)
	if err != nil {
		return err
	}

	// clone component
	err = clone("https://github.com/256dpi/esp-mqtt.git", filepath.Join(buildTreeDir, "components", "esp-mqtt"), ESPMQTTVersion, out)
	if err != nil {
		return err
	}

	// clone component
	err = clone("https://github.com/shiftr-io/naos-esp.git", filepath.Join(buildTreeDir, "components", "naos-esp"), ESPLibVersion, out)
	if err != nil {
		return err
	}

	return nil
}

// BuildTreeLocation returns the location of the build tree if it exists or an
// error if it does not exist.
func (p *Project) BuildTreeLocation() (string, error) {
	// calculate directory
	dir := filepath.Join(p.HiddenDirectory(), "tree")

	// check if build tree directory exists
	ok, err := exists(dir)
	if err != nil {
		return "", err
	} else if !ok {
		return "", nil
	}

	return dir, nil
}

// Build will build the project.
func (p *Project) Build(appOnly bool, out io.Writer) error {
	// build project (app only)
	if appOnly {
		err := p.exec(out, nil, "make", "app")
		if err != nil {
			return err
		}

		return nil
	}

	// build project
	err := p.exec(out, nil, "make", "all")
	if err != nil {
		return err
	}

	return nil
}

// Flash will flash the project to the attached device.
func (p *Project) Flash(erase bool, appOnly bool, out io.Writer) error {
	// get hidden directory
	developmentFramework, err := p.DevelopmentFrameworkLocation()
	if err != nil {
		return err
	}

	// get build tree location
	buildTree, err := p.BuildTreeLocation()
	if err != nil {
		return err
	}

	// calculate paths
	espTool := filepath.Join(developmentFramework, "components", "esptool_py", "esptool", "esptool.py")
	bootLoaderBinary := filepath.Join(buildTree, "build", "bootloader", "bootloader.bin")
	projectBinary := filepath.Join(buildTree, "build", "naos-project.bin")
	partitionsBinary := filepath.Join(buildTree, "build", "partitions_two_ota.bin")

	// prepare erase flash command
	eraseFlash := []string{
		espTool,
		"--chip", "esp32",
		"--port", "/dev/cu.SLAB_USBtoUART",
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"erase_flash",
	}

	// prepare flash all command
	flashAll := []string{
		espTool,
		"--chip", "esp32",
		"--port", "/dev/cu.SLAB_USBtoUART",
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash",
		"-z",
		"--flash_mode", "dio",
		"--flash_freq", "40m",
		"--flash_size", "detect",
		"0x1000", bootLoaderBinary,
		"0x10000", projectBinary,
		"0x8000", partitionsBinary,
	}

	// prepare flash app command
	flashApp := []string{
		espTool,
		"--chip", "esp32",
		"--port", "/dev/cu.SLAB_USBtoUART",
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"write_flash",
		"-z",
		"--flash_mode", "dio",
		"--flash_freq", "40m",
		"--flash_size", "detect",
		"0x10000", projectBinary,
	}

	// erase attached device if requested
	if erase {
		err := p.exec(out, nil, "python", eraseFlash...)
		if err != nil {
			return err
		}
	}

	// flash attached device (app only)
	if appOnly {
		err := p.exec(out, nil, "python", flashApp...)
		if err != nil {
			return err
		}

		return nil
	}

	// flash attached device
	err = p.exec(out, nil, "python", flashAll...)
	if err != nil {
		return err
	}

	return nil
}

// Attach will attach to the attached device.
func (p *Project) Attach(out io.Writer, in io.Reader) error {
	// get toolchain location
	toolchain, err := p.ToolchainLocation()
	if err != nil {
		return err
	}

	// get build tree location
	buildTree, err := p.BuildTreeLocation()
	if err != nil {
		return err
	}

	// construct command
	cmd := exec.Command("make", "simple_monitor")

	// set working directory
	cmd.Dir = buildTree

	// connect output and inputs
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = in

	// inherit current environment
	cmd.Env = os.Environ()

	// go through all env variables
	for i, str := range cmd.Env {
		if strings.HasPrefix(str, "PATH=") {
			// prepend toolchain bin directory
			cmd.Env[i] = "PATH=" + filepath.Join(toolchain, "bin") + ":" + os.Getenv("PATH")
		} else if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + buildTree
		}
	}

	// start process and get tty
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	// read and write data until EOF
	go io.Copy(os.Stdin, tty)
	io.Copy(os.Stdout, tty)
	tty.Close()

	return nil
}

func (p *Project) exec(out io.Writer, in io.Reader, name string, arg ...string) error {
	// get toolchain location
	toolchain, err := p.ToolchainLocation()
	if err != nil {
		return err
	}

	// get build tree location
	buildTree, err := p.BuildTreeLocation()
	if err != nil {
		return err
	}

	// construct command
	cmd := exec.Command(name, arg...)

	// set working directory
	cmd.Dir = buildTree

	// connect output and inputs
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = in

	// inherit current environment
	cmd.Env = os.Environ()

	// go through all env variables
	for i, str := range cmd.Env {
		if strings.HasPrefix(str, "PATH=") {
			// prepend toolchain bin directory
			cmd.Env[i] = "PATH=" + filepath.Join(toolchain, "bin") + ":" + os.Getenv("PATH")
		} else if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + buildTree
		}
	}

	// run command
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
