package tree

import (
	"io"

	"github.com/256dpi/naos/pkg/utils"
)

// Attach will attach to the specified serial port using the idf.py monitor.
func Attach(naosPath, port string, noReset bool, out io.Writer, in io.Reader) error {
	// get idf major version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// log, as the monitor receives keys unaltered, Ctrl+C is forwarded to the
	// device and Ctrl+] is what leaves the monitor
	utils.Log(out, "Attaching to serial port (press Ctrl+] to exit)...")

	// prepare arguments
	args := []string{"monitor"}
	if idfMajorVersion == 4 {
		args = append(args, "-B", "115200")
	} else {
		args = append(args, "-b", "115200")
	}
	if noReset {
		args = append(args, "--no-reset")
	}
	args = append(args, "-p", port)

	// run monitor
	err = Exec(naosPath, out, in, false, true, "idf.py", args...)
	if err != nil {
		return err
	}

	return nil
}
