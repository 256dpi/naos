package tree

import (
	"io"
	"path/filepath"

	"github.com/256dpi/naos/pkg/utils"
)

// Attach will attach to the specified serial port using the idf.py monitor.
func Attach(naosPath, port string, out io.Writer, in io.Reader) error {
	// get idf major version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// log
	utils.Log(out, "Attaching to serial port (press Ctrl+C to exit)...")

	// run monitor
	if idfMajorVersion == 3 {
		tool := filepath.Join(IDFDirectory(naosPath), "tools", "idf_monitor.py")
		elf := filepath.Join(Directory(naosPath), "build", "naos-project.elf")
		err = Exec(naosPath, out, in, true, "python", tool, "--baud", "115200", "--port", port, elf)

	} else {
		err = Exec(naosPath, out, in, true, "idf.py", "monitor", "-b", "115200", "-p", port)
	}
	if err != nil {
		return err
	}

	return nil
}
