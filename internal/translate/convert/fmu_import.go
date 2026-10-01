//go:build !sysml_prod && !sysml_nofmi

package convert

import (
	"bytes"
	"os"
	"path/filepath"

	execfmi "github.com/Open-MBEE/OpenSysML/internal/exec/fmi"
	tfmi "github.com/Open-MBEE/OpenSysML/internal/translate/fmi"
)

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
