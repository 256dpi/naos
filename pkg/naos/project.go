package naos

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/256dpi/naos/pkg/fleet"
	"github.com/256dpi/naos/pkg/tree"
	"github.com/256dpi/naos/pkg/utils"
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

		// update CMake file anyway
		utils.Log(out, "Ensuring project CMake file.")
		err = ioutil.WriteFile(projectPath, []byte(projectCMakeListsFile), 0644)
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

// Install will download necessary dependencies. Any existing dependencies will be
// removed if force is set to true. If out is not nil, it will be used to log
// information about the process.
func (p *Project) Install(force bool, out io.Writer) error {
	// install tree
	err := tree.Install(p.Tree(), filepath.Join(p.Location, "src"), p.Location, p.Inventory.Version, force, p.Inventory.FixSerial, out)
	if err != nil {
		return err
	}

	// install components
	for name, com := range p.Inventory.Components {
		err = tree.InstallComponent(p.Location, p.Tree(), name, com.Path, com.Repository, com.Version, force, out)
		if err != nil {
			return err
		}
	}

	// update cmake lists file
	err = tree.WriteCMakeLists(p.Tree(), out)
	if err != nil {
		return err
	}

	return nil
}

// Build will build the project.
func (p *Project) Build(overrides map[string]string, clean, appOnly bool, out io.Writer) error {
	// merge overrides with inventory overrides
	or := map[string]string{}
	for k, v := range p.Inventory.Overrides {
		or[k] = v
	}
	for k, v := range overrides {
		or[k] = v
	}

	return tree.Build(p.Tree(), or, p.Inventory.Embeds, clean, appOnly, out)
}

// BuildTrace will build the project with tracing enabled.
func (p *Project) BuildTrace(cpuCore, baudRate string, clean, appOnly bool, out io.Writer) error {
	// ensure baud rate
	if baudRate == "" {
		baudRate = p.Inventory.BaudRate
		if baudRate == "" {
			baudRate = "921600"
		}
	}

	// merge overrides with inventory overrides
	or := map[string]string{}
	for k, v := range p.Inventory.Overrides {
		or[k] = v
	}
	for k, v := range map[string]string{
		// app trace
		"CONFIG_APPTRACE_DEST_UART":               "y",
		"CONFIG_APPTRACE_DEST_UART_NOUSB":         "y",
		"CONFIG_APPTRACE_DEST_UART0":              "y",
		"CONFIG_APPTRACE_DEST_UART_NONE":          "",
		"CONFIG_APPTRACE_UART_TX_GPIO":            "1",
		"CONFIG_APPTRACE_UART_RX_GPIO":            "3",
		"CONFIG_APPTRACE_UART_BAUDRATE":           baudRate,
		"CONFIG_APPTRACE_UART_RX_BUFF_SIZE":       "128",
		"CONFIG_APPTRACE_UART_TX_BUFF_SIZE":       "4096",
		"CONFIG_APPTRACE_UART_TX_MSG_SIZE":        "128",
		"CONFIG_APPTRACE_LOCK_ENABLE":             "",
		"CONFIG_APPTRACE_ENABLE":                  "y",
		"CONFIG_APPTRACE_ONPANIC_HOST_FLUSH_TMO":  "-1",
		"CONFIG_APPTRACE_POSTMORTEM_FLUSH_THRESH": "0",
		// system view
		"CONFIG_APPTRACE_SV_ENABLE":                      "y",
		"CONFIG_APPTRACE_SV_DEST_UART":                   "y",
		"CONFIG_APPTRACE_SV_DEST_CPU_" + cpuCore:         "y",
		"CONFIG_APPTRACE_SV_TS_SOURCE_ESP_TIMER":         "y",
		"CONFIG_APPTRACE_SV_MAX_TASKS":                   "32",
		"CONFIG_APPTRACE_SV_BUF_WAIT_TMO":                "500",
		"CONFIG_APPTRACE_SV_EVT_OVERFLOW_ENABLE":         "y",
		"CONFIG_APPTRACE_SV_EVT_ISR_ENTER_ENABLE":        "y",
		"CONFIG_APPTRACE_SV_EVT_ISR_EXIT_ENABLE":         "y",
		"CONFIG_APPTRACE_SV_EVT_ISR_TO_SCHED_ENABLE":     "y",
		"CONFIG_APPTRACE_SV_EVT_TASK_START_EXEC_ENABLE":  "y",
		"CONFIG_APPTRACE_SV_EVT_TASK_STOP_EXEC_ENABLE":   "y",
		"CONFIG_APPTRACE_SV_EVT_TASK_START_READY_ENABLE": "y",
		"CONFIG_APPTRACE_SV_EVT_TASK_STOP_READY_ENABLE":  "y",
		"CONFIG_APPTRACE_SV_EVT_TASK_CREATE_ENABLE":      "y",
		"CONFIG_APPTRACE_SV_EVT_TASK_TERMINATE_ENABLE":   "y",
		"CONFIG_APPTRACE_SV_EVT_IDLE_ENABLE":             "y",
		"CONFIG_APPTRACE_SV_EVT_TIMER_ENTER_ENABLE":      "y",
		"CONFIG_APPTRACE_SV_EVT_TIMER_EXIT_ENABLE":       "y",
		// console
		"CONFIG_ESP_CONSOLE_UART_DEFAULT":  "",
		"CONFIG_ESP_CONSOLE_UART":          "",
		"CONFIG_ESP_CONSOLE_NONE":          "y",
		"CONFIG_ESP_CONSOLE_UART_BAUDRATE": "",
		"CONFIG_ESP_CONSOLE_UART_NUM":      "-1",
		"CONFIG_ESP_IPC_TASK_STACK_SIZE":   "2048",
		// apptrace 2
		"CONFIG_ESP32_APPTRACE_LOCK_ENABLE":                  "",
		"CONFIG_ESP32_APPTRACE_ENABLE":                       "y",
		"CONFIG_ESP32_APPTRACE_ONPANIC_HOST_FLUSH_TMO":       "-1",
		"CONFIG_ESP32_APPTRACE_POSTMORTEM_FLUSH_TRAX_THRESH": "0",
		// sysview 2
		"CONFIG_SYSVIEW_ENABLE":                      "y",
		"CONFIG_SYSVIEW_TS_SOURCE_ESP_TIMER":         "y",
		"CONFIG_SYSVIEW_MAX_TASKS":                   "32",
		"CONFIG_SYSVIEW_BUF_WAIT_TMO":                "500",
		"CONFIG_SYSVIEW_EVT_OVERFLOW_ENABLE":         "y",
		"CONFIG_SYSVIEW_EVT_ISR_ENTER_ENABLE":        "y",
		"CONFIG_SYSVIEW_EVT_ISR_EXIT_ENABLE":         "y",
		"CONFIG_SYSVIEW_EVT_ISR_TO_SCHEDULER_ENABLE": "y",
		"CONFIG_SYSVIEW_EVT_TASK_START_EXEC_ENABLE":  "y",
		"CONFIG_SYSVIEW_EVT_TASK_STOP_EXEC_ENABLE":   "y",
		"CONFIG_SYSVIEW_EVT_TASK_START_READY_ENABLE": "y",
		"CONFIG_SYSVIEW_EVT_TASK_STOP_READY_ENABLE":  "y",
		"CONFIG_SYSVIEW_EVT_TASK_CREATE_ENABLE":      "y",
		"CONFIG_SYSVIEW_EVT_TASK_TERMINATE_ENABLE":   "y",
		"CONFIG_SYSVIEW_EVT_IDLE_ENABLE":             "y",
		"CONFIG_SYSVIEW_EVT_TIMER_ENTER_ENABLE":      "y",
		"CONFIG_SYSVIEW_EVT_TIMER_EXIT_ENABLE":       "y",
		// console 2
		"CONFIG_CONSOLE_UART_DEFAULT":  "",
		"CONFIG_CONSOLE_UART":          "",
		"CONFIG_CONSOLE_UART_BAUDRATE": "",
		"CONFIG_CONSOLE_UART_NONE":     "y",
		"CONFIG_ESP_CONSOLE_UART_NONE": "y",
		"CONFIG_CONSOLE_UART_NUM":      "-1",
		"CONFIG_IPC_TASK_STACK_SIZE":   "2048",
	} {
		or[k] = v
	}

	return tree.Build(p.Tree(), or, p.Inventory.Embeds, clean, appOnly, out)
}

// Flash will flash the project to the attached device.
func (p *Project) Flash(device, baudRate string, erase bool, appOnly bool, out io.Writer) error {
	// ensure baud rate
	if baudRate == "" {
		baudRate = p.Inventory.BaudRate
		if baudRate == "" {
			baudRate = "921600"
		}
	}

	// set missing device
	if device == "" {
		device = utils.FindPort(out)
	}

	return tree.Flash(p.Tree(), device, baudRate, erase, appOnly, out)
}

// Attach will attach to the attached device.
func (p *Project) Attach(device string, out io.Writer, in io.Reader) error {
	// set missing device
	if device == "" {
		device = utils.FindPort(out)
	}

	return tree.Attach(p.Tree(), device, out, in)
}

// Exec will execute a command withing the tree.
func (p *Project) Exec(cmd string, out io.Writer, in io.Reader) error {
	// execute command
	return tree.Exec(p.Tree(), out, in, false, cmd)
}

// Config will write settings and parameters to an attached device.
func (p *Project) Config(file, device string, out io.Writer) error {
	// load file
	data, err := ioutil.ReadFile(file)
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
		device = utils.FindPort(out)
	}

	return tree.Config(p.Tree(), values, device, out)
}

// Format will format all source files in the project if 'clang-format' is
// available.
func (p *Project) Format(out io.Writer) error {
	return tree.Format(p.Tree(), out)
}

// Debug will request coredumps from the devices that match the supplied glob
// pattern. The coredumps are saved to the 'debug' directory in the project.
func (p *Project) Debug(pattern string, delete bool, duration time.Duration, out io.Writer) error {
	// collect coredumps
	coredumps, err := p.Inventory.Debug(pattern, delete, duration)
	if err != nil {
		return err
	}

	// log info
	utils.Log(out, fmt.Sprintf("Got %d coredump(s)", len(coredumps)))

	// ensure directory
	err = os.MkdirAll(filepath.Join(p.Location, "debug"), 0755)
	if err != nil {
		return err
	}

	// go through all coredumps
	for device, coredump := range coredumps {
		// parse coredump
		data, err := tree.ParseCoredump(p.Tree(), coredump)
		if err != nil {
			return err
		}

		// prepare path
		path := filepath.Join(p.Location, "debug", device.Name)

		// write parsed data
		utils.Log(out, fmt.Sprintf("Writing coredump to '%s", path))
		err = ioutil.WriteFile(path, data, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// Update will update the devices that match the supplied glob pattern with the
// previously built image. The specified callback is called for every change in
// state or progress.
func (p *Project) Update(version, pattern string, jobs int, timeout time.Duration, callback func(*Device, *fleet.UpdateStatus)) error {
	// get binary
	bytes, err := tree.AppBinary(p.Tree())
	if err != nil {
		return err
	}

	// run update
	err = p.Inventory.Update(version, pattern, bytes, jobs, timeout, callback)
	if err != nil {
		return err
	}

	return nil
}
