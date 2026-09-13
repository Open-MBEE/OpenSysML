package analysis

import (
	"errors"
	"fmt"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
)

// JobsEnvVar names the variable setting how many runs of one plan go concurrently.
const JobsEnvVar = "OPENSYSML_JOBS"

// DefaultJobs is the jobs a plan has when nothing sets them: one per CPU.
func DefaultJobs() int { return goruntime.NumCPU() }

// ErrJobs is the typed error for a jobs setting that is not a positive integer.
var ErrJobs = errors.New("jobs must be a positive integer")

// JobsError reports a jobs setting no plan can run under: the value as written and where
// it was written, the variable or the flag.
type JobsError struct {
	Source string
	Value  string
}

// Error names the source and the value, and what a usable one is.
func (e *JobsError) Error() string {
	return fmt.Sprintf("%s=%q is not a positive integer: set it to how many runs may go concurrently (default %d, one per CPU)", e.Source, e.Value, DefaultJobs())
}

// Is matches ErrJobs.
func (e *JobsError) Is(target error) bool { return target == ErrJobs }

// ParseJobs reads a jobs setting written at source: a positive integer, else a JobsError.
func ParseJobs(source, text string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n <= 0 {
		return 0, &JobsError{Source: source, Value: text}
	}
	return n, nil
}

// JobsFromEnv is the jobs OPENSYSML_JOBS asks for, DefaultJobs when it is unset or empty;
// a value that is not a positive integer is reported rather than left at the default.
func JobsFromEnv() (int, error) {
	return jobsFromLookup(os.Getenv)
}

// jobsFromLookup is JobsFromEnv over an explicit lookup, so the parsing is testable
// without the process environment.
func jobsFromLookup(lookup func(string) string) (int, error) {
	raw := lookup(JobsEnvVar)
	if strings.TrimSpace(raw) == "" {
		return DefaultJobs(), nil
	}
	return ParseJobs(JobsEnvVar, raw)
}
