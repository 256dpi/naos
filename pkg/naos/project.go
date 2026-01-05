package naos

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/256dpi/naos/pkg/serial"
	"github.com/256dpi/naos/pkg/tree"
	"github.com/256dpi/naos/pkg/utils"
)

// A Project is a project available on disk.
type Project struct {
	Location string
	Manifest *Manifest
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

	// check existence
	ok, err := utils.Exists(filepath.Join(path, "naos.json"))
	if err != nil {
		return nil, err
	}

	// create new manifest if not already exists
	if !ok || force {
		// create empty manifest
		p.Manifest = NewManifest()

		// save manifest
		utils.Log(out, "Created new empty manifest.")
		err = p.SaveManifest()
		if err != nil {
			return nil, err
		}
	} else {
		// open existing project
		utils.Log(out, "Manifest already exists.")
		p, err = OpenProject(path)
		if err != nil {
			return nil, err
		}
	}

	// ensure source directory
	utils.Log(out, "Ensuring source directory.")
	err = os.MkdirAll(filepath.Join(path, "src"), 0755)
	if err != nil {
		return nil, err
	}

	// prepare main source path and check if it already exists
	utils.Log(out, "Creating default source file.")
	mainSourcePath := filepath.Join(path, "src", "main.c")
	err = utils.Ensure(mainSourcePath, mainSourceFile, force)
	if err != nil {
		return nil, err
	}

	// generate cmake file if requested
	if cmake {
		utils.Log(out, "Creating CMakeLists file.")
		cMakeListsPath := filepath.Join(p.Location, "CMakeLists.txt")
		err = utils.Ensure(cMakeListsPath, projectCMakeListsFile, force)
		if err != nil {
			return nil, err
		}
	}

	// create ".gitignore" file
	gitIgnoreEntries := []string{"naos/"}
	if cmake {
		gitIgnoreEntries = append(gitIgnoreEntries, "cmake-build-debug/")
	}
	gitIgnoreData := []byte(strings.Join(gitIgnoreEntries, "\n") + "\n")
	gitignorePath := filepath.Join(p.Location, ".gitignore")
	err = utils.Ensure(gitignorePath, string(gitIgnoreData), force)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// OpenProject will open the project in the specified path.
func OpenProject(path string) (*Project, error) {
	// attempt to read manifest
	man, err := ReadManifest(filepath.Join(path, "naos.json"))
	if err != nil {
		return nil, err
	}

	// prepare project
	project := &Project{
		Location: path,
		Manifest: man,
	}

	return project, nil
}

// SaveManifest will save the associated manifest to disk.
func (p *Project) SaveManifest() error {
	// save manifest
	err := p.Manifest.Save(filepath.Join(p.Location, "naos.json"))
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
	// install tree
	err := tree.Install(p.Tree(), filepath.Join(p.Location, "src"), filepath.Join(p.Location, "data"), p.Manifest.Version, force, out)
	if err != nil {
		return err
	}

	// install non-registry components
	for name, com := range p.Manifest.Components {
		if com.Registry == "" {
			err = tree.InstallComponent(p.Location, p.Tree(), name, com.Path, com.Repository, com.Version, force, out)
			if err != nil {
				return err
			}
		}
	}

	// install registry components
	var registryComponents []tree.IDFComponent
	for _, com := range p.Manifest.Components {
		if com.Registry != "" {
			registryComponents = append(registryComponents, tree.IDFComponent{
				Name:       com.Registry,
				Version:    com.Version,
				Repository: com.Repository,
				Path:       com.Path,
			})
		}
	}
	err = tree.InstallRegistryComponents(p.Location, p.Tree(), registryComponents, force, out)
	if err != nil {
		return err
	}

	// install audio framework
	err = tree.InstallAudioFramework(p.Tree(), p.Manifest.Frameworks.Audio, force, out)
	if err != nil {
		return err
	}

	// TODO: Move to "build"?

	// update cmake lists file
	err = tree.WriteCMakeLists(p.Tree(), out)
	if err != nil {
		return err
	}

	return nil
}

// Build will build the project.
func (p *Project) Build(clean, reconfigure, appOnly bool, out io.Writer) error {
	// execute command
	return tree.Build(p.Tree(), p.Manifest.Name, p.Manifest.TagPrefix, p.Manifest.Target, p.Manifest.Overrides, p.Manifest.Embeds, p.Manifest.Partitions, clean, reconfigure, appOnly, out)
}

// Flash will flash the project to the attached device.
func (p *Project) Flash(device, baudRate string, erase bool, appOnly, alt bool, out io.Writer) error {
	// ensure baud rate
	if baudRate == "" {
		baudRate = p.Manifest.BaudRate
		if baudRate == "" {
			baudRate = "921600"
		}
	}

	// set missing device
	if device == "" {
		device = serial.FindPort()
	}

	return tree.Flash(p.Tree(), p.Manifest.Name, p.Manifest.Target, device, baudRate, erase, appOnly, alt, out)
}

// Attach will attach to the attached device.
func (p *Project) Attach(device string, out io.Writer, in io.Reader) error {
	// set missing device
	if device == "" {
		device = serial.FindPort()
	}

	return tree.Attach(p.Tree(), device, out, in)
}

// Exec will execute a command withing the tree.
func (p *Project) Exec(cmd string, out io.Writer, in io.Reader) error {
	// execute command
	return tree.Exec(p.Tree(), out, in, false, false, cmd)
}

// Config will write settings and parameters to an attached device.
func (p *Project) Config(file, device, baudRate string, out io.Writer) error {
	// ensure baud rate
	if baudRate == "" {
		baudRate = p.Manifest.BaudRate
		if baudRate == "" {
			baudRate = "921600"
		}
	}

	// load file
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// unmarshal values
	var values map[string]string
	err = yaml.Unmarshal(data, &values)
	if err != nil {
		return err
	}

	// set missing device
	if device == "" {
		device = serial.FindPort()
	}

	return tree.Config(p.Tree(), values, device, baudRate, out)
}

// Format will format all source files in the project if 'clang-format' is
// available.
func (p *Project) Format(out io.Writer) error {
	return tree.Format(p.Tree(), out)
}

// Bundle will create a bundle of the project.
func (p *Project) Bundle(file string, out io.Writer) error {
	return tree.Bundle(p.Tree(), file, out)
}
