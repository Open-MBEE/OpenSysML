// Package gobuild builds a command of this module for a test that drives it as
// a process, so the coverage profile still credits what the process ran.
package gobuild

import "os"

// modulePattern names every package of this module for -coverpkg.
const modulePattern = "github.com/Open-MBEE/OpenSysML/..."

// DirEnv names the directory instrumented binaries write their counters to.
// `make coverage` sets it and folds the counters into coverage.txt.
const DirEnv = "OPENSYSML_GOCOVERDIR"

// runtimeDirEnv is the variable the Go runtime reads for the same purpose.
const runtimeDirEnv = "GOCOVERDIR"

// Args returns the `go build` arguments that write the package in the working directory
// to out; under `make coverage` the binary is instrumented and its counters collected.
func Args(out string, extra ...string) []string {
	args := []string{"build"}
	if dir := Dir(); dir != "" {
		// -pgo=off as in make coverage: cmd/*/default.pgo plus -cover trips golang/go#80891.
		args = append(args, "-cover", "-covermode=atomic", "-coverpkg="+modulePattern, "-pgo=off")
		_ = os.Setenv(runtimeDirEnv, dir)
	}
	args = append(args, extra...)
	return append(args, "-o", out, ".")
}

// Dir returns the counter directory of the coverage run, or "" outside one.
func Dir() string {
	return os.Getenv(DirEnv)
}
