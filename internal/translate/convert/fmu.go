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

// FMUChangedError reports an FMU import whose name resolves a file that does
// not hold the bytes converted: the archive moved between the read and the
// uri the calc def would carry.
type FMUChangedError struct{ Name string }

// Error names the file that changed.
func (e *FMUChangedError) Error() string {
	return fmt.Sprintf("the FMU at %s is not the archive that was converted; re-read it and convert again", e.Name)
}

// Is matches ErrFMUChanged.
func (e *FMUChangedError) Is(target error) bool { return target == ErrFMUChanged }

// ErrFMUChanged is the typed error for an FMU whose file changed since it was read.
var ErrFMUChanged = errors.New("the FMU changed since it was read")

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
	// #nosec G304 -- the file is the one the user named for conversion.
	if onDisk, err := os.ReadFile(abs); err != nil || !bytes.Equal(onDisk, data) {
		return nil, &FMUChangedError{Name: name}
	}
	return tfmi.Notation(d, tfmi.Options{URI: execfmi.FileURI(abs)})
}
