// Package hostcap reports what the host a build runs on can do beyond the language
// itself, so a capability that is not there is named rather than failing obscurely.
package hostcap

import (
	"errors"
	"fmt"
)

// ErrSpawnUnsupported is the cause every external-process operation reports on a host
// that can start no process: every Go WebAssembly target (GOOS=js and GOOS=wasip1,
// both GOARCH=wasm). Callers reach it through errors.Is, wrapped into whichever typed
// error their layer answers with.
var ErrSpawnUnsupported = errors.New("a WebAssembly build cannot start external processes")

// UnsupportedError names what cannot run on this host.
type UnsupportedError struct {
	// Subject is what was to run: a solver, an engine's executable, a compiler.
	Subject string
}

// Error names the subject and the cause it cannot run for.
func (e *UnsupportedError) Error() string {
	if e.Subject == "" {
		return ErrSpawnUnsupported.Error()
	}
	return fmt.Sprintf("%s cannot run: %s", e.Subject, ErrSpawnUnsupported)
}

// Unwrap returns ErrSpawnUnsupported.
func (e *UnsupportedError) Unwrap() error { return ErrSpawnUnsupported }
