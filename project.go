package naos

import "os"

// A Project is a project available on disk.
type Project struct {
	Location  string
	Inventory *Inventory
}

// CreateProject will initialize a project in the specified directory with the
// specified name.
func CreateProject(path, name string) (*Project, error) {
	// ensure project directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	// create project
	project := &Project{
		Location:  path,
		Inventory: &Inventory{},
	}

	// save inventory
	err = project.Inventory.Save(path)
	if err != nil {
		return nil, err
	}

	// TODO: Create "main.c".
	// TODO: Create CMakeLists.txt.

	return project, nil
}

// FindProject will look for project in the specified path.
func FindProject(path string) (*Project, error) {
	// attempt to read inventory
	inv, err := ReadInventory(path)
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
