// Package gobuild builds a command of this module for a test that drives it as
// a process, so the coverage profile still credits what the process ran.
package gobuild

import (
	"fmt"
	"os"
	"sync"
)

// modulePattern names every package of this module for -coverpkg.
const modulePattern = "github.com/Open-MBEE/OpenSysML/..."

// DirEnv names the directory instrumented binaries write their counters to.
// `make coverage` sets it and folds the counters into coverage.txt.
const DirEnv = "OPENSYSML_GOCOVERDIR"

// runtimeDirEnv is the variable the Go runtime reads for the same purpose.
const runtimeDirEnv = "GOCOVERDIR"

// A coverage run's counter directory is handed to the Go runtime once, before any test
// runs, so every instrumented process the tests start inherits it and no helper changes
// the environment while tests are running.
func init() {
	if err := exportDir(); err != nil {
		panic(err)
	}
}

// exportDir sets the runtime's variable to the coverage run's counter directory.
func exportDir() error {
	dir := Dir()
	if dir == "" {
		return nil
	}
	if err := os.Setenv(runtimeDirEnv, dir); err != nil {
		return fmt.Errorf("gobuild: set %s for instrumented commands: %w", runtimeDirEnv, err)
	}
	return nil
}

// Args returns the `go build` arguments that write the package in the working directory
// to out; under `make coverage` the binary is instrumented. It changes no environment.
func Args(out string, extra ...string) []string {
	args := []string{"build"}
	if Dir() != "" {
		// -pgo=off as in make coverage: cmd/*/default.pgo plus -cover trips golang/go#80891.
		args = append(args, "-cover", "-covermode=atomic", "-coverpkg="+modulePattern, "-pgo=off")
	}
	args = append(args, extra...)
	return append(args, "-o", out, ".")
}

// Dir returns the counter directory of the coverage run, or "" outside one.
func Dir() string {
	return os.Getenv(DirEnv)
}

// Scratch is a directory a test binary builds its commands into once; the package's
// TestMain removes it after the run, so a cached build outlives no test binary.
type Scratch struct {
	// Pattern names the directory, as os.MkdirTemp reads it.
	Pattern string

	once sync.Once
	dir  string
	err  error
}

// Dir returns the directory, made on first use.
func (s *Scratch) Dir() (string, error) {
	s.once.Do(func() { s.dir, s.err = os.MkdirTemp("", s.Pattern) })
	return s.dir, s.err
}

// Remove deletes the directory and what was built into it; a Scratch never used has none.
func (s *Scratch) Remove() error {
	s.once.Do(func() {})
	if s.dir == "" {
		return nil
	}
	return os.RemoveAll(s.dir)
}
