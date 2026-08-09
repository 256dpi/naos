package tree

import (
	"bytes"
	"io"
	"regexp"
)

// closingMessage matches the first line of the message idf.py prints after a
// build, which suggests flashing commands that do not apply to a NAOS project.
var closingMessage = regexp.MustCompile(`^(Project|App|Partition Table|Bootloader) build complete\.`)

// closingFilter drops the closing message printed by idf.py, and everything
// that follows it, as it is the last thing printed.
type closingFilter struct {
	out    io.Writer
	rest   []byte
	blanks int
	done   bool
}

// filterClosingMessage will wrap the provided writer to drop the closing
// message printed by idf.py after a build.
func filterClosingMessage(out io.Writer) *closingFilter {
	return &closingFilter{out: out}
}

func (f *closingFilter) Write(buf []byte) (int, error) {
	// ignore data once the message has been reached
	if f.done {
		return len(buf), nil
	}

	// buffer data
	f.rest = append(f.rest, buf...)

	// handle complete lines
	for {
		// find line
		i := bytes.IndexByte(f.rest, '\n')
		if i < 0 {
			break
		}
		line := f.rest[:i+1]
		f.rest = f.rest[i+1:]

		// stop at the message, dropping the blank line preceding it
		if closingMessage.Match(line) {
			f.blanks = 0
			f.done = true
			return len(buf), nil
		}

		// withhold blank lines until it is known that they do not precede the
		// message
		if len(bytes.TrimSpace(line)) == 0 {
			f.blanks++
			continue
		}

		// write line
		err := f.write(line)
		if err != nil {
			return 0, err
		}
	}

	return len(buf), nil
}

// Flush will write any withheld data.
func (f *closingFilter) Flush() error {
	// check state
	if f.done || (f.blanks == 0 && len(f.rest) == 0) {
		return nil
	}

	// write rest
	rest := f.rest
	f.rest = nil

	return f.write(rest)
}

// write will write the withheld blank lines followed by the provided data.
func (f *closingFilter) write(buf []byte) error {
	// check writer
	if f.out == nil {
		f.blanks = 0
		return nil
	}

	// write withheld blank lines
	for ; f.blanks > 0; f.blanks-- {
		_, err := f.out.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}

	// write data
	_, err := f.out.Write(buf)

	return err
}
