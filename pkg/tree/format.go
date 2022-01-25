package tree

import "io"

// Format will format all source files in the project if 'clang-format' is
// available.
func Format(naosPath string, out io.Writer) error {
	// get source and header files
	sourceFiles, headerFiles, err := SourceAndHeaderFiles(naosPath)
	if err != nil {
		return err
	}

	// prepare arguments
	arguments := []string{"-style", "{BasedOnStyle: Google, ColumnLimit: 120, SortIncludes: false}", "-i"}
	arguments = append(arguments, sourceFiles...)
	arguments = append(arguments, headerFiles...)

	// format source files
	err = Exec(naosPath, out, nil, "clang-format", arguments...)
	if err != nil {
		return err
	}

	return nil
}
