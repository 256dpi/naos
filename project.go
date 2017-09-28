package naos

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

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
func CreateProject(path string, force, cmake bool, out io.Writer) (*Project, error) {
	// ensure project directory
	utils.Log(out, "Ensuring project directory.")
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	// create project
	p := &Project{Location: path}

	// check if inventory already exists
	ok, err := utils.Exists(filepath.Join(path, "naos.json"))
	if err != nil {
		return nil, err
	}

	// create new inventory if not already exists
	if !ok || force {
		// create empty inventory
		p.Inventory = NewInventory()

		// save inventory
		utils.Log(out, "Created new empty inventory.")
		err = p.SaveInventory()
		if err != nil {
			return nil, err
		}
	} else {
		utils.Log(out, "Inventory already exists.")
	}

	// ensure source directory
	utils.Log(out, "Ensuring source directory.")
	err = os.MkdirAll(filepath.Join(path, "src"), 0755)
	if err != nil {
		return nil, err
	}

	// prepare main source path and check if it already exists
	mainSourcePath := filepath.Join(path, "src", "main.c")
	ok, err = utils.Exists(mainSourcePath)
	if err != nil {
		return nil, err
	}

	// create main source file if it not already exists
	if !ok || force {
		utils.Log(out, "Creating default source file.")
		err = ioutil.WriteFile(mainSourcePath, []byte(mainSourceFile), 0644)
		if err != nil {
			return nil, err
		}
	} else {
		utils.Log(out, "Default source file already exists.")
	}

	// generate cmake file if requested
	if cmake {
		// get project path
		projectPath := filepath.Join(p.Location, "CMakeLists.txt")
		ok, err := utils.Exists(projectPath)
		if err != nil {
			return nil, err
		}

		// create CMake file if it not already exists
		if !ok || force {
			utils.Log(out, "Creating project CMake file.")
			err = ioutil.WriteFile(projectPath, []byte(projectCMakeListsFile), 0644)
			if err != nil {
				return nil, err
			}
		} else {
			utils.Log(out, "Project CMake file already exists.")
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

// Install will download necessary dependencies. Any existing dependencies will be
// removed if force is set to true. If out is not nil, it will be used to log
// information about the process.
func (p *Project) Install(force bool, out io.Writer) error {
	return tree.Install(p.Tree(), filepath.Join(p.Location, "src"), "master", force, out)
}

// Build will build the project.
func (p *Project) Build(clean, appOnly bool, out io.Writer) error {
	return tree.Build(p.Tree(), clean, appOnly, out)
}

// Flash will flash the project to the attached device.
func (p *Project) Flash(device string, erase bool, appOnly bool, out io.Writer) error {
	return tree.Flash(p.Tree(), device, erase, appOnly, out)
}

// Attach will attach to the attached device.
func (p *Project) Attach(device string, simple bool, out io.Writer, in io.Reader) error {
	return tree.Attach(p.Tree(), device, simple, out, in)
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
	err = tree.Exec(p.Tree(), out, nil, "clang-format", arguments...)
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
	image := filepath.Join(p.Tree(), "build", "naos-project.bin")

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
