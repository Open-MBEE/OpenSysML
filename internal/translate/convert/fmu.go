package convert

import (
	"errors"
	"fmt"
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
