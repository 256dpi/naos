package tree

import (
	"io"

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
	if idfMajorVersion == 4 {
		err = Exec(naosPath, out, in, false, true, "idf.py", "monitor", "-B", "115200", "-p", port)
	} else {
		err = Exec(naosPath, out, in, false, true, "idf.py", "monitor", "-b", "115200", "-p", port)
	}
	if err != nil {
		return err
	}

	return nil
}
