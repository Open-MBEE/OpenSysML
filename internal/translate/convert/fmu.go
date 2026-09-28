package convert

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	execfmi "github.com/Open-MBEE/OpenSysML/internal/exec/fmi"
	tfmi "github.com/Open-MBEE/OpenSysML/internal/translate/fmi"
)

// FMUNotAFileError reports an FMU import whose name resolves no file on disk:
// the archive came from standard input or an inline document, and the calc
// def's uri must name the archive, so there is nothing correct to write.
type FMUNotAFileError struct{ Name string }

// Error names where the FMU came from.
func (e *FMUNotAFileError) Error() string {
	return fmt.Sprintf("the FMU came from %s, not a file; the calc's uri must name the archive, so convert from a file path", e.Name)
}

// Is matches ErrFMUNotAFile.
func (e *FMUNotAFileError) Is(target error) bool { return target == ErrFMUNotAFile }

// ErrFMUNotAFile is the typed error for an FMU import that names no file.
var ErrFMUNotAFile = errors.New("the FMU did not come from a file")

// fmuToNotation reads an FMU's model description and writes the notation
// importing it, the ToolExecution uri naming the file the FMU came from.
func fmuToNotation(name string, data []byte) ([]byte, error) {
	d, err := execfmi.ReadArchive(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, &FMUNotAFileError{Name: name}
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, &FMUNotAFileError{Name: name}
	}
	return tfmi.Notation(d, tfmi.Options{URI: "file://" + filepath.ToSlash(abs)})
}
