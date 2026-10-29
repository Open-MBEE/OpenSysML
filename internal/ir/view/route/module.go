// Package route routes diagram edges orthogonally with libavoid, the router the
// VS Code diagram panel uses, run as WebAssembly inside the Go process.
package route

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
)

// EnvVar names an alternative libavoid WebAssembly module to load in place of
// the embedded one, so the LGPL-licensed router stays replaceable.
const EnvVar = "OPENSYSML_LIBAVOID"

// embeddedModule is libavoid-js 0.4.5's libavoid.wasm with its legacy
// exception-handling instructions translated to the standard exnref form.
//
//go:embed libavoid.wasm
var embeddedModule []byte

// ModuleError reports a libavoid module that could not be loaded or did not
// behave as the router expects; Source names where the module came from.
type ModuleError struct {
	Source string
	Err    error
}

func (e *ModuleError) Error() string {
	return fmt.Sprintf("libavoid module %s: %v", e.Source, e.Err)
}

func (e *ModuleError) Unwrap() error { return e.Err }

// ErrNoRoute reports an edge libavoid could not route between its ends.
var ErrNoRoute = errors.New("no orthogonal route")

// moduleBytes returns the module to run and where it came from: the file named
// by OPENSYSML_LIBAVOID when set, else the embedded copy.
func moduleBytes() ([]byte, string, error) {
	path := os.Getenv(EnvVar)
	if path == "" {
		return embeddedModule, "embedded", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, &ModuleError{Source: path, Err: err}
	}
	return data, path, nil
}
