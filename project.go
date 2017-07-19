package naos

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr/pty"
	"github.com/shiftr-io/naos/mqtt"
	"github.com/shiftr-io/naos/tree"
	"github.com/shiftr-io/naos/utils"
)

// A Project is a project available on disk.
type Project struct {
	Location  string
	Inventory *Inventory
}

// CreateProject will initialize a project in the specified directory. If out is
// not nil, it will be used to log information about the process.
func CreateProject(path string, out io.Writer) (*Project, error) {
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

	// print info
	utils.Log(out, "Created new empty inventory.")
	utils.Log(out, "Please update the settings to suit your needs.")

	// ensure source directory
	utils.Log(out, "Ensuring source directory.")
	err = os.MkdirAll(filepath.Join(path, "src"), 0755)
	if err != nil {
		return nil, err
	}

	// prepare main source path and check if it already exists
	mainSourcePath := filepath.Join(path, "src", "main.c")
	ok, err := utils.Exists(mainSourcePath)
	if err != nil {
		return nil, err
	}

	// create main source file if it not already exists
	if !ok {
		utils.Log(out, "Creating default 'main.c' source file.")
		err = ioutil.WriteFile(mainSourcePath, []byte(mainSourceFile), 0644)
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}

// OpenProject will open the project in the specified path.
func OpenProject(path string) (*Project, error) {
	// attempt to read inventory
	inv, err := ReadInventory(filepath.Join(path, "naos.json"))
	if err != nil {
		return nil, err
	}

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
	err := p.Inventory.Save(filepath.Join(p.Location, "naos.json"))
	if err != nil {
		return err
	}

	return nil
}

// Tree returns the internal directory used to store the toolchain, development
// framework and other necessary files.
func (p *Project) Tree() string {
	return filepath.Join(p.Location, "naos")
}

// Setup will setup necessary dependencies. Any existing dependencies will be
// removed if force is set to true. If out is not nil, it will be used to log
// information about the process.
func (p *Project) Setup(force bool, cmake bool, out io.Writer) error {
	// install build tree
	tree.Install(p.Tree(), filepath.Join(p.Location, "src"), "master", true, out)

	// TODO: Make cmake stuff to create.

	// generate cmake file if requested
	if cmake {
		// get project path
		projectPath := filepath.Join(p.Location, "CMakeLists.txt")
		ok, err := utils.Exists(projectPath)
		if err != nil {
			return err
		}

		if !ok || force {
			utils.Log(out, "Creating project CMake file.")
			err = ioutil.WriteFile(projectPath, []byte(projectCMakeListsFile), 0644)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// TODO: Move all build tree commands to tree package.

// Build will build the project.
func (p *Project) Build(clean, appOnly bool, out io.Writer) error {
	// clean project if requested
	if clean {
		utils.Log(out, "Cleaning project...")
		err := p.exec(out, nil, "make", "clean")
		if err != nil {
			return err
		}
	}

	// build project (app only)
	if appOnly {
		utils.Log(out, "Building project (app only)...")
		err := p.exec(out, nil, "make", "app")
		if err != nil {
			return err
		}

		return nil
	}

	// build project
	utils.Log(out, "Building project...")
	err := p.exec(out, nil, "make", "all")
	if err != nil {
		return err
	}

	return nil
}

// Flash will flash the project to the attached device.
func (p *Project) Flash(device string, erase bool, appOnly bool, out io.Writer) error {
	// calculate paths
	espTool := filepath.Join(tree.IDFDirectory(p.Tree()), "components", "esptool_py", "esptool", "esptool.py")
	bootLoaderBinary := filepath.Join(p.Tree(), "build", "bootloader", "bootloader.bin")
	projectBinary := filepath.Join(p.Tree(), "build", "naos-project.bin")
	partitionsBinary := filepath.Join(p.Tree(), "build", "partitions_two_ota.bin")

	// prepare erase flash command
	eraseFlash := []string{
		espTool,
		"--chip", "esp32",
		"--port", device,
		"--baud", "921600",
		"--before", "default_reset",
		"--after", "hard_reset",
		"erase_flash",
	}

	// prepare flash all command
	flashAll := []string{
		espTool,
		"--chip", "esp32",
		"--port", device,
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
		"--port", device,
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
		utils.Log(out, "Erasing flash...")
		err := p.exec(out, nil, "python", eraseFlash...)
		if err != nil {
			return err
		}
	}

	// flash attached device (app only)
	if appOnly {
		utils.Log(out, "Flashing device (app only)...")
		err := p.exec(out, nil, "python", flashApp...)
		if err != nil {
			return err
		}

		return nil
	}

	// flash attached device
	utils.Log(out, "Flashing device...")
	err := p.exec(out, nil, "python", flashAll...)
	if err != nil {
		return err
	}

	return nil
}

// Attach will attach to the attached device.
func (p *Project) Attach(device string, simple bool, out io.Writer, in io.Reader) error {
	// prepare command
	var cmd *exec.Cmd

	// set simple or advanced command
	if simple {
		// construct command
		cmd = exec.Command("miniterm.py", "--rts", "0", "--dtr", "0", "--raw", device, "115200")
	} else {
		// get path of monitor tool
		tool := filepath.Join(tree.IDFDirectory(p.Tree()), "tools", "idf_monitor.py")

		// get elf path
		elf := filepath.Join(p.Tree(), "build", "naos-project.elf")

		// construct command
		cmd = exec.Command("python", tool, "--baud", "115200", "--port", device, elf)
	}

	// set working directory
	cmd.Dir = p.Tree()

	// inherit current environment
	cmd.Env = os.Environ()

	// go through all env variables
	for i, str := range cmd.Env {
		if strings.HasPrefix(str, "PATH=") {
			// prepend toolchain bin directory
			cmd.Env[i] = "PATH=" + tree.BinDirectory(p.Tree()) + ":" + os.Getenv("PATH")
		} else if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + p.Tree()
		}
	}

	// start process and get tty
	utils.Log(out, "Attaching to device (press Ctrl+C to exit)...")
	tty, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	// make sure tty gets closed
	defer tty.Close()

	// prepare channel
	quit := make(chan struct{})

	// read data until EOF
	go func() {
		io.Copy(out, tty)
		close(quit)
	}()

	// write data until EOF
	go func() {
		io.Copy(tty, in)
		close(quit)
	}()

	// wait for quit
	<-quit

	return nil
}

// Format will format all source files in the project if 'clang-format' is
// available.
func (p *Project) Format(out io.Writer) error {
	// get source and header files
	sourceFiles, headerFiles, err := p.sourceAndHeaderFiles()
	if err != nil {
		return err
	}

	// prepare arguments
	arguments := []string{"-style", "{BasedOnStyle: Google, ColumnLimit: 120}", "-i"}
	arguments = append(arguments, sourceFiles...)
	arguments = append(arguments, headerFiles...)

	// format source files
	err = p.exec(out, nil, "clang-format", arguments...)
	if err != nil {
		return err
	}

	return nil
}

// Update will update the devices that match the supplied glob pattern with the
// previously built image. The specified callback is called for every change in
// state or progress.
func (p *Project) Update(pattern string, timeout time.Duration, callback func(*Device, *mqtt.UpdateStatus)) error {
	// get image path
	image := filepath.Join(p.Tree(), "tree", "build", "naos-project.bin")

	// read image
	bytes, err := ioutil.ReadFile(image)
	if err != nil {
		return err
	}

	// run update
	err = p.Inventory.Update(pattern, bytes, timeout, callback)
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) exec(out io.Writer, in io.Reader, name string, arg ...string) error {
	// construct command
	cmd := exec.Command(name, arg...)

	// set working directory
	cmd.Dir = p.Tree()

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
			cmd.Env[i] = "PATH=" + tree.BinDirectory(p.Tree()) + ":" + os.Getenv("PATH")
		} else if strings.HasPrefix(str, "PWD=") {
			// override shell working directory
			cmd.Env[i] = "PWD=" + p.Tree()
		}
	}

	// run command
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) sourceAndHeaderFiles() ([]string, []string, error) {
	// prepare list
	sourceFiles := make([]string, 0)
	headerFiles := make([]string, 0)

	// scan directory
	err := filepath.Walk(filepath.Join(p.Location, "src"), func(path string, f os.FileInfo, err error) error {
		// directly return errors
		if err != nil {
			return err
		}

		// add files with matching extension
		if filepath.Ext(path) == ".c" {
			sourceFiles = append(sourceFiles, path)
		} else if filepath.Ext(path) == ".h" {
			headerFiles = append(headerFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sourceFiles, headerFiles, nil
}
