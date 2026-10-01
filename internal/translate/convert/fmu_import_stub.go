//go:build sysml_prod || sysml_nofmi

package convert

import "errors"

// ErrFMINotLinked is the answer to an FMU input in a build made without the
// FMI model import.
var ErrFMINotLinked = errors.New("FMI model import is not available in this build (built without fmi)")

func fmuToNotation(string, []byte) ([]byte, error) { return nil, ErrFMINotLinked }
