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
	uri := name
	if abs, err := filepath.Abs(name); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			uri = "file://" + filepath.ToSlash(abs)
		}
	}
	return tfmi.Notation(d, tfmi.Options{URI: uri})
}
