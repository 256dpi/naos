package tree

import (
	"bytes"
	"io"
	"os"

	"github.com/256dpi/naos/pkg/utils"
)

// quitKeyReader translates Ctrl+C into the monitor's quit key. The terminal is
// put in raw mode while attached, so Ctrl+C is not turned into an interrupt
// signal anymore and would otherwise just be forwarded to the device.
type quitKeyReader struct {
	input io.Reader
}

// Terminal implements the terminalInput interface.
func (r *quitKeyReader) Terminal() *os.File {
	file, _ := r.input.(*os.File)
	return file
}

func (r *quitKeyReader) Read(buf []byte) (int, error) {
	// read data
	n, err := r.input.Read(buf)

	// replace interrupts with the quit key
	data := buf[:n]
	for {
		i := bytes.IndexByte(data, 0x03)
		if i < 0 {
			break
		}
		data[i] = 0x1d
		data = data[i+1:]
	}

	return n, err
}

// Attach will attach to the specified serial port using the idf.py monitor.
func Attach(naosPath, port string, noReset bool, out io.Writer, in io.Reader) error {
	// get idf major version
	idfMajorVersion, err := IDFMajorVersion(naosPath)
	if err != nil {
		return err
	}

	// log
	utils.Log(out, "Attaching to serial port (press Ctrl+C or Ctrl+] to exit)...")

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
	err = Exec(naosPath, out, &quitKeyReader{input: in}, false, true, "idf.py", args...)
	if err != nil {
		return err
	}

	return nil
}
